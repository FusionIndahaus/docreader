package integrations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	amocrmpkg "document-ai/internal/integrations/amocrm"
	bitrixpkg "document-ai/internal/integrations/bitrix"
	onecpkg "document-ai/internal/integrations/onec"
)

type Dispatcher struct {
	DB             *sql.DB
	AmoRedirectURI string
}

func (d Dispatcher) Dispatch(userEmail, text string) {
	if d.DB == nil {
		return
	}
	email := strings.ToLower(strings.TrimSpace(userEmail))
	var customerID string
	row := d.DB.QueryRow(`select id from customers where deleted_at is null and lower(email)=lower($1)`, email)
	if scanErr := row.Scan(&customerID); scanErr != nil || customerID == "" {
		log.Printf("⚠️ [Dispatcher] customer not found for email=%q scanErr=%v", email, scanErr)
		return
	}
	log.Printf("🔀 [Dispatcher] found customerID=%s for email=%s", customerID, email)

	// Читаем флаги enabled напрямую из user_integrations
	amocrmEnabled := false
	bitrixEnabled := false
	onecEnabled := false
	rows, err := d.DB.Query(`select provider, enabled from user_integrations where user_id=$1 and provider in ('amocrm','bitrix','1c')`, customerID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var provider string
			var enabled bool
			if rows.Scan(&provider, &enabled) == nil && enabled {
				switch provider {
				case "amocrm":
					amocrmEnabled = true
				case "bitrix":
					bitrixEnabled = true
				case "1c":
					onecEnabled = true
				}
			}
		}
	}
	log.Printf("🔀 [Dispatcher] email=%s amocrm=%v bitrix=%v 1c=%v", email, amocrmEnabled, bitrixEnabled, onecEnabled)

	// amoCRM
	if amocrmEnabled && amocrmpkg.IsConfigured() {
		var cfgJSON string
		row = d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='amocrm'`, customerID)
		_ = row.Scan(&cfgJSON)
		lines := strings.Split(text, "\n")
		lead := map[string]interface{}{
			"name": truncate("Document AI: "+text, 200),
		}
		type cfVal struct {
			FieldID int64                    `json:"field_id"`
			Values  []map[string]interface{} `json:"values"`
		}
		var customFields []cfVal
		if cfgJSON != "" {
			var conf map[string]interface{}
			if err := json.Unmarshal([]byte(cfgJSON), &conf); err == nil {
				if raw, ok := conf["lead_field_map_by_index"]; ok {
					if mp, ok2 := raw.(map[string]interface{}); ok2 {
						for idxStr, dest := range mp {
							destStr, _ := dest.(string)
							if destStr == "" {
								continue
							}
							if i, err := strconv.Atoi(idxStr); err == nil && i > 0 && i <= len(lines) {
								val := strings.TrimSpace(lines[i-1])
								if val == "" {
									continue
								}
								if destStr == "name" {
									lead["name"] = truncate(val, 200)
									continue
								}
								if destStr == "price" {
									lead["price"] = val
									continue
								}
								if strings.HasPrefix(destStr, "cf:") {
									idStr := strings.TrimPrefix(destStr, "cf:")
									if fid, err := strconv.ParseInt(idStr, 10, 64); err == nil && fid > 0 {
										customFields = append(customFields, cfVal{
											FieldID: fid,
											Values:  []map[string]interface{}{{"value": val}},
										})
									}
								}
							}
						}
					}
				}
			}
		}
		if len(customFields) > 0 {
			lead["custom_fields_values"] = customFields
		}
		bodyBytes, _ := json.Marshal([]interface{}{lead})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if resp, err := amocrmpkg.APIRequestForCustomer(ctx, d.DB, customerID, http.MethodPost, "/api/v4/leads", bytes.NewReader(bodyBytes), d.AmoRedirectURI); err == nil && resp != nil {
			_ = resp.Body.Close()
		}
	}
	// Bitrix24
	if bitrixEnabled {
		var webhookBase, cfgJSON string
		row = d.DB.QueryRow(`select coalesce(account_domain,''), coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, customerID)
		_ = row.Scan(&webhookBase, &cfgJSON)
		webhookBase = strings.TrimRight(strings.TrimSpace(webhookBase), "/")
		if webhookBase != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var conf map[string]interface{}
			_ = json.Unmarshal([]byte(cfgJSON), &conf)
			if conf == nil {
				conf = map[string]interface{}{}
			}
			entity := "lead"
			if v, ok := conf["entity_type"].(string); ok && (v == "deal" || v == "lead") {
				entity = v
			}
			fields := map[string]interface{}{
				"TITLE":    truncate("Document AI", 128),
				"COMMENTS": truncate(text, 4000),
			}
			key := "lead_field_map_by_index"
			if entity == "deal" {
				key = "deal_field_map_by_index"
			}
			if raw, ok := conf[key]; ok {
				if mp, ok2 := raw.(map[string]interface{}); ok2 {
					lines := strings.Split(text, "\n")
					for idxStr, fieldCodeRaw := range mp {
						fieldCode, _ := fieldCodeRaw.(string)
						if fieldCode == "" {
							continue
						}
						if i, err := strconv.Atoi(idxStr); err == nil && i > 0 && i <= len(lines) {
							val := strings.TrimSpace(lines[i-1])
							if val != "" {
								fields[fieldCode] = val
							}
						}
					}
				}
			}
			if entity == "deal" {
				err := bitrixpkg.SendDeal(ctx, webhookBase, fields)
				if err != nil {
					log.Printf("❌ [Dispatcher] Bitrix SendDeal error: %v", err)
				} else {
					log.Printf("✅ [Dispatcher] Bitrix deal created successfully")
				}
			} else {
				err := bitrixpkg.SendLead(ctx, webhookBase, fields)
				if err != nil {
					log.Printf("❌ [Dispatcher] Bitrix SendLead error: %v", err)
				} else {
					log.Printf("✅ [Dispatcher] Bitrix lead created successfully")
				}
			}
		}
	}
	// 1C (BYOA)
	if onecEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cfg, _ := onecpkg.LoadUserConfig(ctx, d.DB, customerID)
		if strings.TrimSpace(cfg.BaseURL) != "" {
			cred, _ := onecpkg.LoadCredentials(ctx, d.DB, customerID)
			lines := strings.Split(text, "\n")
			payload := map[string]interface{}{
				"text":  text,
				"lines": lines,
			}
			var cfgJSON string
			row = d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='1c'`, customerID)
			_ = row.Scan(&cfgJSON)
			if cfgJSON != "" {
				var conf map[string]interface{}
				if err := json.Unmarshal([]byte(cfgJSON), &conf); err == nil {
					if raw, ok := conf["json_field_map_by_index"]; ok {
						if mp, ok2 := raw.(map[string]interface{}); ok2 {
							for idxStr, fieldKeyRaw := range mp {
								fieldKey, _ := fieldKeyRaw.(string)
								if fieldKey == "" {
									continue
								}
								if i, err := strconv.Atoi(idxStr); err == nil && i > 0 && i <= len(lines) {
									val := strings.TrimSpace(lines[i-1])
									if val != "" {
										payload[fieldKey] = val
									}
								}
							}
						}
					}
				}
			}
			_ = onecpkg.SendJSON(ctx, cfg, cred, payload)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
