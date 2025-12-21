package integrations

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	amocrmpkg "document-ai/internal/integrations/amocrm"
	bitrixpkg "document-ai/internal/integrations/bitrix"
)

type Dispatcher struct {
	DB             *sql.DB
	AmoRedirectURI string
}

func (d Dispatcher) Dispatch(userEmail, text string) {
	if d.DB == nil {
		return
	}
	var metadata string
	row := d.DB.QueryRow(`SELECT metadata FROM customers WHERE deleted_at IS NULL AND lower(email)=lower($1)`, userEmail)
	if err := row.Scan(&metadata); err != nil {
		return
	}
	var settings map[string]interface{}
	if metadata != "" {
		_ = json.Unmarshal([]byte(metadata), &settings)
	}
	amocrmEnabled := false
	bitrixEnabled := false
	if v, ok := settings["destinations"]; ok {
		if m, ok2 := v.(map[string]interface{}); ok2 {
			if raw, ok3 := m["amocrm"]; ok3 {
				if b, ok4 := raw.(bool); ok4 && b {
					amocrmEnabled = true
				}
			}
			if raw, ok3 := m["bitrix"]; ok3 {
				if b, ok4 := raw.(bool); ok4 && b {
					bitrixEnabled = true
				}
			}
		}
	}
	// customer ID
	email := strings.ToLower(strings.TrimSpace(userEmail))
	var customerID string
	row = d.DB.QueryRow(`select id from customers where deleted_at is null and lower(email)=lower($1)`, email)
	if err := row.Scan(&customerID); err != nil || customerID == "" {
		return
	}

	// amoCRM
	if amocrmEnabled && amocrmpkg.IsConfigured() {
		var enabled bool
		row = d.DB.QueryRow(`select enabled from user_integrations where user_id=$1 and provider='amocrm'`, customerID)
		_ = row.Scan(&enabled)
		if enabled {
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
	}
	// Bitrix24
	if bitrixEnabled {
		cfg, _ := bitrixpkg.LoadUserConfig(context.Background(), d.DB, customerID)
		if cfg.Enabled && strings.TrimSpace(cfg.WebhookBase) != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var cfgEnabled bool
			var cfgJSON string
			row := d.DB.QueryRow(`select enabled, coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, customerID)
			_ = row.Scan(&cfgEnabled, &cfgJSON)
			entity := "lead"
			if cfgJSON != "" {
				var conf map[string]interface{}
				_ = json.Unmarshal([]byte(cfgJSON), &conf)
				if v, ok := conf["entity_type"].(string); ok && (v == "deal" || v == "lead") {
					entity = v
				}
			}
			fields := map[string]interface{}{
				"TITLE":    truncate("Document AI", 128),
				"COMMENTS": truncate(text, 4000),
			}
			if cfgEnabled && cfgJSON != "" {
				var conf map[string]interface{}
				if err := json.Unmarshal([]byte(cfgJSON), &conf); err == nil {
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
				}
			}
			if entity == "deal" {
				_ = bitrixpkg.SendDeal(ctx, cfg.WebhookBase, fields)
			} else {
				_ = bitrixpkg.SendLead(ctx, cfg.WebhookBase, fields)
			}
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
