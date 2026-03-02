package handlers

import (
	"context"
	"database/sql"
	amocrmpkg "document-ai/internal/integrations/amocrm"
	bitrixpkg "document-ai/internal/integrations/bitrix"
	"document-ai/internal/http/middleware"
	onecpkg "document-ai/internal/integrations/onec"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type IntegrationsDeps struct {
	DB               *sql.DB
	RequireUser      func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse func(http.ResponseWriter, interface{})
	SendJSONError    func(http.ResponseWriter, string, int)
	GetUserEmail     func(string) string
	AmoRedirectURI   string
}

func RegisterIntegrationUserRoutes(mux *http.ServeMux, d IntegrationsDeps) {
	// AmoCRM settings
	mux.HandleFunc("/user/integrations/amocrm/settings", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserAmoSettingsGet(w, r, d)
		case http.MethodPut:
			handleUserAmoSettingsPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	// AmoCRM fields
	mux.HandleFunc("/user/integrations/amocrm/fields", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		handleUserAmoFields(w, r, d)
	}))
	// AmoCRM mapping
	mux.HandleFunc("/user/integrations/amocrm/mapping", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserAmoMappingGet(w, r, d)
		case http.MethodPut:
			handleUserAmoMappingPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Bitrix settings
	mux.HandleFunc("/user/integrations/bitrix/settings", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserBitrixSettingsGet(w, r, d)
		case http.MethodPut:
			handleUserBitrixSettingsPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	// Bitrix fields
	mux.HandleFunc("/user/integrations/bitrix/fields", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		handleUserBitrixFields(w, r, d)
	}))
	// Bitrix mapping
	mux.HandleFunc("/user/integrations/bitrix/mapping", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserBitrixMappingGet(w, r, d)
		case http.MethodPut:
			handleUserBitrixMappingPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// 1C settings
	mux.HandleFunc("/user/integrations/1c/settings", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserOneCSettingsGet(w, r, d)
		case http.MethodPut:
			handleUserOneCSettingsPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	// 1C mapping
	mux.HandleFunc("/user/integrations/1c/mapping", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleUserOneCMappingGet(w, r, d)
		case http.MethodPut:
			handleUserOneCMappingPut(w, r, d)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}

func handleUserBitrixSettingsGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var enabled bool
	var accountDomain string
	var confJSON string
	row := d.DB.QueryRow(`
		SELECT enabled, COALESCE(account_domain,''), COALESCE(config::text,'{}')
		FROM user_integrations
		WHERE user_id=$1 AND provider='bitrix'
	`, userID)
	_ = row.Scan(&enabled, &accountDomain, &confJSON)
	entityType := "lead"
	if confJSON != "" {
		var conf map[string]interface{}
		_ = json.Unmarshal([]byte(confJSON), &conf)
		if v, ok := conf["entity_type"].(string); ok && (v == "deal" || v == "lead") {
			entityType = v
		}
	}
	d.SendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":      enabled,
			"webhook_base": strings.TrimSpace(accountDomain),
			"entity_type":  entityType,
		},
	})
}

func handleUserBitrixSettingsPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	log.Printf("🔍 [Bitrix Settings PUT] userID from context: %q", userID)
	
	if userID == "" {
		log.Printf("❌ [Bitrix Settings PUT] userID is empty, returning unauthorized")
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("✅ [Bitrix Settings PUT] userID validated")
	
	if d.DB == nil {
		log.Printf("❌ [Bitrix Settings PUT] DB is nil")
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	log.Printf("✅ [Bitrix Settings PUT] DB connection validated")
	
	var p struct {
		Enabled     *bool  `json:"enabled"`
		WebhookBase string `json:"webhook_base"`
		EntityType  string `json:"entity_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		log.Printf("❌ [Bitrix Settings PUT] JSON decode error: %v", err)
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("✅ [Bitrix Settings PUT] Parsed request: enabled=%v, webhook_base=%q, entity_type=%q", 
		p.Enabled, p.WebhookBase, p.EntityType)
	
	webhookBase := strings.TrimRight(strings.TrimSpace(p.WebhookBase), "/")
	if webhookBase == "" {
		log.Printf("❌ [Bitrix Settings PUT] webhook_base is empty after trimming")
		d.SendJSONError(w, "webhook_base is required", http.StatusBadRequest)
		return
	}
	log.Printf("✅ [Bitrix Settings PUT] webhook_base validated: %q", webhookBase)
	
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	log.Printf("✅ [Bitrix Settings PUT] enabled: %v", enabled)
	
	var currentConfStr string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, userID)
	_ = row.Scan(&currentConfStr)
	log.Printf("✅ [Bitrix Settings PUT] Current config: %q", currentConfStr)
	
	var conf map[string]interface{}
	if currentConfStr != "" {
		_ = json.Unmarshal([]byte(currentConfStr), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	et := strings.ToLower(strings.TrimSpace(p.EntityType))
	if et != "deal" {
		et = "lead"
	}
	conf["entity_type"] = et
	confJSON, _ := json.Marshal(conf)
	log.Printf("✅ [Bitrix Settings PUT] Config JSON: %s", string(confJSON))
	
	log.Printf("🔄 [Bitrix Settings PUT] Executing INSERT/UPDATE query with userID=%q, enabled=%v, webhookBase=%q, config=%s", 
		userID, enabled, webhookBase, string(confJSON))
	
	_, err := d.DB.Exec(`
		INSERT INTO user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		VALUES(gen_random_uuid(), $1, 'bitrix', $2, $3, '{}'::jsonb, $4::jsonb)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			enabled        = EXCLUDED.enabled,
			account_domain = EXCLUDED.account_domain,
			config         = EXCLUDED.config,
			updated_at     = now()
	`, userID, enabled, webhookBase, string(confJSON))
	if err != nil {
		log.Printf("❌ [Bitrix Settings PUT] DB Exec error: %v", err)
		d.SendJSONError(w, "save failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ [Bitrix Settings PUT] Successfully saved to database")
	
	d.SendJSONResponse(w, APIResponse{Status: "success"})
	log.Printf("✅ [Bitrix Settings PUT] Response sent successfully")
}

func handleUserAmoSettingsGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var enabled bool
	var accountDomain string
	var credentialsJSON string
	row := d.DB.QueryRow(`
		SELECT enabled, COALESCE(account_domain,''), COALESCE(credentials::text,'{}')
		FROM user_integrations
		WHERE user_id=$1 AND provider='amocrm'
	`, userID)
	_ = row.Scan(&enabled, &accountDomain, &credentialsJSON)

	var creds map[string]interface{}
	_ = json.Unmarshal([]byte(credentialsJSON), &creds)
	if _, ok := creds["client_secret"]; ok {
		creds["client_secret"] = "***"
	}
	d.SendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":        enabled,
			"account_domain": accountDomain,
			"credentials":    creds,
		},
	})
}

func handleUserAmoSettingsPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var p struct {
		Enabled       *bool   `json:"enabled"`
		AccountDomain string  `json:"account_domain"`
		ClientID      string  `json:"client_id"`
		ClientSecret  string  `json:"client_secret"`
		RedirectURI   *string `json:"redirect_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
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
		d.SendJSONError(w, "account_domain, client_id, client_secret are required", http.StatusBadRequest)
		return
	}
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
	_, err := d.DB.Exec(`
		INSERT INTO user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		VALUES(gen_random_uuid(), $1, 'amocrm', $2, $3, $4::jsonb, '{}'::jsonb)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			enabled        = EXCLUDED.enabled,
			account_domain = EXCLUDED.account_domain,
			credentials    = EXCLUDED.credentials,
			updated_at     = now()
	`, userID, enabled, accountDomain, string(credsJSON))
	if err != nil {
		d.SendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleUserBitrixFields(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	cfg, err := bitrixpkg.LoadUserConfig(r.Context(), d.DB, userID)
	if err != nil || strings.TrimSpace(cfg.WebhookBase) == "" {
		d.SendJSONError(w, "bitrix not configured", http.StatusBadRequest)
		return
	}
	log.Printf("🔍 [Bitrix Fields] userID=%q webhook_base=%q", userID, cfg.WebhookBase)
	var confJSON string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, userID)
	_ = row.Scan(&confJSON)
	entity := "lead"
	if confJSON != "" {
		var conf map[string]interface{}
		_ = json.Unmarshal([]byte(confJSON), &conf)
		if v, ok := conf["entity_type"].(string); ok && (v == "deal" || v == "lead") {
			entity = v
		}
	}
	log.Printf("🔍 [Bitrix Fields] entity_type=%q, fetching fields from Bitrix API...", entity)
	var fields []bitrixpkg.Field
	if entity == "deal" {
		fields, err = bitrixpkg.FetchDealFields(r.Context(), cfg.WebhookBase)
	} else {
		fields, err = bitrixpkg.FetchLeadFields(r.Context(), cfg.WebhookBase)
	}
	if err != nil {
		log.Printf("❌ [Bitrix Fields] failed to fetch fields from %q: %v", cfg.WebhookBase, err)
		d.SendJSONError(w, "failed to fetch fields from Bitrix: "+err.Error(), http.StatusBadGateway)
		return
	}
	log.Printf("✅ [Bitrix Fields] fetched %d fields", len(fields))
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: fields})
}

func handleUserBitrixMappingGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var configJSON string
	row := d.DB.QueryRow(`
		select coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='bitrix'
	`, userID)
	_ = row.Scan(&configJSON)
	var conf map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &conf)
	if conf == nil {
		conf = map[string]interface{}{}
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: conf})
}

func handleUserBitrixMappingPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var current string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='bitrix'`, userID)
	_ = row.Scan(&current)
	var conf map[string]interface{}
	if current != "" {
		_ = json.Unmarshal([]byte(current), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	if et, ok := body["entity_type"].(string); ok {
		et = strings.ToLower(strings.TrimSpace(et))
		if et == "deal" || et == "lead" {
			conf["entity_type"] = et
		}
	}
	if m, ok := body["lead_field_map_by_index"]; ok {
		conf["lead_field_map_by_index"] = m
	}
	if m, ok := body["deal_field_map_by_index"]; ok {
		conf["deal_field_map_by_index"] = m
	}
	confJSON, _ := json.Marshal(conf)
	_, err := d.DB.Exec(`
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, 'bitrix', true, '', '{}'::jsonb, $2::jsonb)
		on conflict (user_id, provider) do update set
			config = excluded.config,
			updated_at = now()
	`, userID, string(confJSON))
	if err != nil {
		d.SendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleUserAmoFields(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := amocrmpkg.APIRequestForCustomer(ctx, d.DB, userID, http.MethodGet, "/api/v4/leads/custom_fields", nil, d.AmoRedirectURI)
	if err != nil || resp == nil {
		d.SendJSONError(w, "failed to fetch amo fields", http.StatusBadGateway)
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
		d.SendJSONError(w, "invalid amo response", http.StatusBadGateway)
		return
	}
	type amoField struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		Scope string `json:"scope"`
	}
	fields := make([]amoField, 0, len(raw.Embedded.CustomFields)+2)
	fields = append(fields, amoField{ID: -1, Name: "name (Название сделки)", Type: "standard", Scope: "standard"})
	fields = append(fields, amoField{ID: -2, Name: "price (Бюджет)", Type: "standard", Scope: "standard"})
	for _, f := range raw.Embedded.CustomFields {
		fields = append(fields, amoField{ID: f.ID, Name: f.Name, Type: f.Type, Scope: "custom"})
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: fields})
}

func handleUserAmoMappingGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var configJSON string
	row := d.DB.QueryRow(`
		select coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='amocrm'
	`, userID)
	_ = row.Scan(&configJSON)
	var conf map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &conf)
	if conf == nil {
		conf = map[string]interface{}{}
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: conf})
}

func handleUserAmoMappingPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var current string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='amocrm'`, userID)
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
	_, err := d.DB.Exec(`
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, 'amocrm', true, '', '{}'::jsonb, $2::jsonb)
		on conflict (user_id, provider) do update set
			config = excluded.config,
			updated_at = now()
	`, userID, string(confJSON))
	if err != nil {
		d.SendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

// ----- 1C (BYOA) -----

func handleUserOneCSettingsGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cfg, _ := onecpkg.LoadUserConfig(ctx, d.DB, userID)
	cred, _ := onecpkg.LoadCredentials(ctx, d.DB, userID)
	// маскируем секреты
	respCred := map[string]interface{}{
		"auth_type": cred.AuthType,
	}
	if cred.Username != "" {
		respCred["username"] = cred.Username
	}
	if cred.Password != "" {
		respCred["password"] = "***"
	}
	if cred.Token != "" {
		respCred["token"] = "***"
	}
	d.SendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"enabled":       cfg.Enabled,
			"base_url":      cfg.BaseURL,
			"endpoint_path": cfg.EndpointPath,
			"credentials":   respCred,
		},
	})
}

func handleUserOneCSettingsPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var p struct {
		Enabled      *bool   `json:"enabled"`
		BaseURL      string  `json:"base_url"`
		EndpointPath *string `json:"endpoint_path"`
		AuthType     string  `json:"auth_type"` // none|basic|bearer
		Username     string  `json:"username"`
		Password     string  `json:"password"`
		Token        string  `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" {
		d.SendJSONError(w, "base_url is required", http.StatusBadRequest)
		return
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	// merge config
	var currentConfStr string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='1c'`, userID)
	_ = row.Scan(&currentConfStr)
	var conf map[string]interface{}
	if currentConfStr != "" {
		_ = json.Unmarshal([]byte(currentConfStr), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	if p.EndpointPath != nil {
		conf["endpoint_path"] = strings.TrimSpace(*p.EndpointPath)
	}
	confJSON, _ := json.Marshal(conf)
	// credentials
	authType := strings.ToLower(strings.TrimSpace(p.AuthType))
	if authType == "" {
		authType = "none"
	}
	creds := map[string]interface{}{
		"auth_type": authType,
	}
	if authType == "basic" {
		creds["username"] = strings.TrimSpace(p.Username)
		creds["password"] = p.Password
	} else if authType == "bearer" {
		creds["token"] = p.Token
	}
	credsJSON, _ := json.Marshal(creds)
	_, err := d.DB.Exec(`
		INSERT INTO user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		VALUES(gen_random_uuid(), $1, '1c', $2, $3, $4::jsonb, $5::jsonb)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			enabled        = EXCLUDED.enabled,
			account_domain = EXCLUDED.account_domain,
			credentials    = EXCLUDED.credentials,
			config         = EXCLUDED.config,
			updated_at     = now()
	`, userID, enabled, baseURL, string(credsJSON), string(confJSON))
	if err != nil {
		d.SendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleUserOneCMappingGet(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var configJSON string
	row := d.DB.QueryRow(`
		select coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='1c'
	`, userID)
	_ = row.Scan(&configJSON)
	var conf map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &conf)
	if conf == nil {
		conf = map[string]interface{}{}
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: conf})
}

func handleUserOneCMappingPut(w http.ResponseWriter, r *http.Request, d IntegrationsDeps) {
	userID := middleware.UserIDFromContext(r)
	if userID == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var current string
	row := d.DB.QueryRow(`select coalesce(config::text,'{}') from user_integrations where user_id=$1 and provider='1c'`, userID)
	_ = row.Scan(&current)
	var conf map[string]interface{}
	if current != "" {
		_ = json.Unmarshal([]byte(current), &conf)
	}
	if conf == nil {
		conf = map[string]interface{}{}
	}
	if m, ok := body["json_field_map_by_index"]; ok {
		conf["json_field_map_by_index"] = m
	}
	if ep, ok := body["endpoint_path"].(string); ok {
		conf["endpoint_path"] = strings.TrimSpace(ep)
	}
	confJSON, _ := json.Marshal(conf)
	_, err := d.DB.Exec(`
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, '1c', true, '', '{}'::jsonb, $2::jsonb)
		on conflict (user_id, provider) do update set
			config = excluded.config,
			updated_at = now()
	`, userID, string(confJSON))
	if err != nil {
		d.SendJSONError(w, "save failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}
