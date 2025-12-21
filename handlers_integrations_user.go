package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// GET /user/integrations/amocrm/settings
func handleUserAmoSettingsGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	// Прочитаем user_integrations для amocrm
	var enabled bool
	var accountDomain string
	var credentialsJSON string
	row := db.QueryRow(`
		SELECT enabled, COALESCE(account_domain,''), COALESCE(credentials::text,'{}')
		FROM user_integrations
		WHERE user_id=$1 AND provider='amocrm'
	`, customerID)
	_ = row.Scan(&enabled, &accountDomain, &credentialsJSON)

	var creds map[string]interface{}
	_ = json.Unmarshal([]byte(credentialsJSON), &creds)
	// Не выдаём секрет полностью
	if _, ok := creds["client_secret"]; ok {
		creds["client_secret"] = "***"
	}
	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":        enabled,
			"account_domain": accountDomain,
			"credentials":    creds,
		},
	})
}

// PUT /user/integrations/amocrm/settings
func handleUserAmoSettingsPut(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var p struct {
		Enabled       *bool   `json:"enabled"`
		AccountDomain string  `json:"account_domain"`
		ClientID      string  `json:"client_id"`
		ClientSecret  string  `json:"client_secret"`
		RedirectURI   *string `json:"redirect_uri"` // можно не задавать, если используем общий callback
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	accountDomain := strings.TrimSpace(p.AccountDomain)
	clientID := strings.TrimSpace(p.ClientID)
	clientSecret := strings.TrimSpace(p.ClientSecret)
	var redirectURI string
	if p.RedirectURI != nil {
		redirectURI = strings.TrimSpace(*p.RedirectURI)
	}
	if accountDomain == "" || clientID == "" || clientSecret == "" {
		sendJSONError(w, "account_domain, client_id, client_secret are required", http.StatusBadRequest)
		return
	}
	// Соберём credentials JSON
	creds := map[string]interface{}{
		"client_id":     clientID,
		"client_secret": clientSecret,
	}
	if redirectURI != "" {
		creds["redirect_uri"] = redirectURI
	}
	credsJSON, _ := json.Marshal(creds)

	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	_, err = db.Exec(`
		INSERT INTO user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		VALUES(gen_random_uuid(), $1, 'amocrm', $2, $3, $4::jsonb, '{}'::jsonb)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			enabled        = EXCLUDED.enabled,
			account_domain = EXCLUDED.account_domain,
			credentials    = EXCLUDED.credentials,
			updated_at     = now()
	`, customerID, enabled, accountDomain, string(credsJSON))
	if err != nil {
		sendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

// GET /user/integrations/bitrix/fields
func handleUserBitrixFields(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	cfg, err := loadCustomerBitrixConfig(r.Context(), customerID)
	if err != nil || strings.TrimSpace(cfg.WebhookBase) == "" {
		sendJSONError(w, "bitrix not configured", http.StatusBadRequest)
		return
	}
	fields, err := fetchBitrixLeadFields(r.Context(), cfg.WebhookBase)
	if err != nil {
		sendJSONError(w, "failed to fetch fields", http.StatusBadGateway)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: fields})
}

// GET /user/integrations/bitrix/mapping
// Возвращает сохранённый маппинг индексов значений -> кодов полей
func handleUserBitrixMappingGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var configJSON string
	row := db.QueryRow(`
		select coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='bitrix'
	`, customerID)
	_ = row.Scan(&configJSON)
	var conf map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &conf)
	if conf == nil {
		conf = map[string]interface{}{}
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: conf})
}

// PUT /user/integrations/bitrix/mapping
// Принимает JSON: { "lead_field_map_by_index": { "1":"UF_CRM_...", "2":"OPPORTUNITY" } }
func handleUserBitrixMappingPut(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	// Мержим в существующий config
	var current string
	row := db.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, customerID)
	_ = row.Scan(&current)
	var conf map[string]interface{}
	if current != "" {
		_ = json.Unmarshal([]byte(current), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	if m, ok := body["lead_field_map_by_index"]; ok {
		conf["lead_field_map_by_index"] = m
	}
	confJSON, _ := json.Marshal(conf)
	_, err = db.Exec(`
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, 'bitrix', true, '', '{}'::jsonb, $2::jsonb)
		on conflict (user_id, provider) do update set
			config = excluded.config,
			updated_at = now()
	`, customerID, string(confJSON))
	if err != nil {
		sendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

// ===== amoCRM fields/mapping (BYOA) =====

type amoField struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Scope string `json:"scope"`
}

// GET /user/integrations/amocrm/fields
func handleUserAmoFields(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := amoAPIRequestForCustomer(ctx, customerID, http.MethodGet, "/api/v4/leads/custom_fields", nil)
	if err != nil || resp == nil {
		sendJSONError(w, "failed to fetch amo fields", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var raw struct {
		Embedded struct {
			CustomFields []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"custom_fields"`
		} `json:"_embedded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		sendJSONError(w, "invalid amo response", http.StatusBadGateway)
		return
	}
	fields := make([]amoField, 0, len(raw.Embedded.CustomFields)+2)
	// Добавим стандартные поля лида, которые можно заполнить напрямую
	fields = append(fields, amoField{ID: -1, Name: "name (Название сделки)", Type: "standard", Scope: "standard"})
	fields = append(fields, amoField{ID: -2, Name: "price (Бюджет)", Type: "standard", Scope: "standard"})
	for _, f := range raw.Embedded.CustomFields {
		fields = append(fields, amoField{ID: f.ID, Name: f.Name, Type: f.Type, Scope: "custom"})
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: fields})
}

// GET /user/integrations/amocrm/mapping
func handleUserAmoMappingGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var configJSON string
	row := db.QueryRow(`
		select coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='amocrm'
	`, customerID)
	_ = row.Scan(&configJSON)
	var conf map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &conf)
	if conf == nil {
		conf = map[string]interface{}{}
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: conf})
}

// PUT /user/integrations/amocrm/mapping
// JSON: { "lead_field_map_by_index": { "1":"name", "2":"price", "3":"cf:12345" } }
func handleUserAmoMappingPut(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var current string
	row := db.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='amocrm'`, customerID)
	_ = row.Scan(&current)
	var conf map[string]interface{}
	if current != "" {
		_ = json.Unmarshal([]byte(current), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	if m, ok := body["lead_field_map_by_index"]; ok {
		conf["lead_field_map_by_index"] = m
	}
	confJSON, _ := json.Marshal(conf)
	_, err = db.Exec(`
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, 'amocrm', true, '', '{}'::jsonb, $2::jsonb)
		on conflict (user_id, provider) do update set
			config = excluded.config,
			updated_at = now()
	`, customerID, string(confJSON))
	if err != nil {
		sendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

// GET /user/integrations/bitrix/settings
func handleUserBitrixSettingsGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var enabled bool
	var accountDomain string
	row := db.QueryRow(`
		SELECT enabled, COALESCE(account_domain,'')
		FROM user_integrations
		WHERE user_id=$1 AND provider='bitrix'
	`, customerID)
	_ = row.Scan(&enabled, &accountDomain)
	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":      enabled,
			"webhook_base": strings.TrimSpace(accountDomain),
		},
	})
}

// PUT /user/integrations/bitrix/settings
func handleUserBitrixSettingsPut(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	var p struct {
		Enabled     *bool  `json:"enabled"`
		WebhookBase string `json:"webhook_base"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	webhookBase := strings.TrimRight(strings.TrimSpace(p.WebhookBase), "/")
	if webhookBase == "" {
		sendJSONError(w, "webhook_base is required", http.StatusBadRequest)
		return
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	_, err = db.Exec(`
		INSERT INTO user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		VALUES(gen_random_uuid(), $1, 'bitrix', $2, $3, '{}'::jsonb, '{}'::jsonb)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			enabled        = EXCLUDED.enabled,
			account_domain = EXCLUDED.account_domain,
			updated_at     = now()
	`, customerID, enabled, webhookBase)
	if err != nil {
		sendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}
