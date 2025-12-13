package main

import (
	"encoding/json"
	"net/http"
	"strings"
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
