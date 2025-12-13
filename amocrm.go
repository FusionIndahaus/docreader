package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type amoTokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type amoUserTokens struct {
	CustomerID    string
	AccountDomain string
	AccessToken   string
	RefreshToken  string
	ExpiresAt     time.Time
}

func isAmoConfigured() bool {
	return true
}

type amoUserCredentials struct {
	AccountDomain string
	ClientID      string
	ClientSecret  string
	RedirectURI   string
}

func loadCustomerAmoCredentials(ctx context.Context, customerID string) (amoUserCredentials, error) {
	var out amoUserCredentials
	if db == nil {
		return out, fmt.Errorf("DB not connected")
	}
	var accountDomain string
	var credentialsJSON []byte
	row := db.QueryRowContext(ctx, `
		select account_domain, credentials
		from user_integrations
		where user_id=$1 and provider='amocrm'
	`, customerID)
	if err := row.Scan(&accountDomain, &credentialsJSON); err != nil {
		return out, err
	}
	out.AccountDomain = strings.TrimRight(strings.TrimSpace(accountDomain), "/")
	var creds map[string]interface{}
	_ = json.Unmarshal(credentialsJSON, &creds)
	if v, ok := creds["client_id"].(string); ok {
		out.ClientID = strings.TrimSpace(v)
	}
	if v, ok := creds["client_secret"].(string); ok {
		out.ClientSecret = strings.TrimSpace(v)
	}
	if v, ok := creds["redirect_uri"].(string); ok {
		out.RedirectURI = strings.TrimSpace(v)
	}
	return out, nil
}

func getAmoAuthorizeURLForCustomer(ctx context.Context, customerID string, state string) (string, error) {
	creds, err := loadCustomerAmoCredentials(ctx, customerID)
	if err != nil {
		return "", err
	}
	if creds.AccountDomain == "" || creds.ClientID == "" {
		return "", fmt.Errorf("amoCRM credentials incomplete")
	}
	redirect := creds.RedirectURI
	// если redirect пуст, можно попытаться использовать глобальный amoRedirectURI, иначе ошибка
	if redirect == "" {
		redirect = strings.TrimSpace(amoRedirectURI)
		if redirect == "" {
			return "", fmt.Errorf("redirect_uri is required")
		}
	}
	base := creds.AccountDomain
	return fmt.Sprintf("%s/oauth2/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		base, urlQueryEscape(creds.ClientID), urlQueryEscape(redirect), urlQueryEscape(state)), nil
}

// ===== Персональные токены amoCRM на пользователя =====

func loadCustomerAmoTokens(ctx context.Context, customerID string) (amoUserTokens, error) {
	var t amoUserTokens
	if db == nil {
		return t, fmt.Errorf("DB not connected")
	}
	// Читаем из user_integrations
	var credentialsJSON []byte
	row := db.QueryRowContext(ctx, `
		select id, user_id, account_domain, credentials
		from user_integrations
		where user_id = $1 and provider = 'amocrm'
	`, customerID)
	var dummyID string
	if err := row.Scan(&dummyID, &t.CustomerID, &t.AccountDomain, &credentialsJSON); err != nil {
		return t, err
	}
	var creds map[string]interface{}
	_ = json.Unmarshal(credentialsJSON, &creds)
	if v, ok := creds["access_token"].(string); ok {
		t.AccessToken = v
	}
	if v, ok := creds["refresh_token"].(string); ok {
		t.RefreshToken = v
	}
	if v, ok := creds["expires_at"].(string); ok && v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			t.ExpiresAt = ts
		}
	}
	return t, nil
}

func saveCustomerAmoTokens(ctx context.Context, customerID string, t amoUserTokens) error {
	if db == nil {
		return fmt.Errorf("DB not connected")
	}
	creds := map[string]interface{}{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
		"expires_at":    t.ExpiresAt.Format(time.RFC3339),
	}
	credsJSON, _ := json.Marshal(creds)
	_, err := db.ExecContext(ctx, `
		insert into user_integrations(id, user_id, provider, enabled, account_domain, credentials, config)
		values(gen_random_uuid(), $1, 'amocrm', true, $2, $3::jsonb, '{}'::jsonb)
		on conflict (user_id, provider) do update set
			enabled       = true,
			account_domain= excluded.account_domain,
			credentials   = excluded.credentials,
			updated_at    = now()
	`, customerID, t.AccountDomain, string(credsJSON))
	return err
}

func ensureCustomerAmoAccessToken(ctx context.Context, customerID string) (string, error) {
	tok, err := loadCustomerAmoTokens(ctx, customerID)
	if err != nil {
		return "", err
	}
	if time.Until(tok.ExpiresAt) < 60*time.Second {
		if err := refreshCustomerAmoToken(ctx, customerID); err != nil {
			return "", err
		}
		tok, err = loadCustomerAmoTokens(ctx, customerID)
		if err != nil {
			return "", err
		}
	}
	return tok.AccessToken, nil
}

func refreshCustomerAmoToken(ctx context.Context, customerID string) error {
	if db == nil {
		return fmt.Errorf("DB not connected")
	}
	if !isAmoConfigured() {
		return fmt.Errorf("amoCRM не сконфигурирован (env)")
	}
	tok, err := loadCustomerAmoTokens(ctx, customerID)
	if err != nil {
		return err
	}
	if tok.RefreshToken == "" {
		return fmt.Errorf("refresh token отсутствует")
	}
	creds, err := loadCustomerAmoCredentials(ctx, customerID)
	if err != nil {
		return err
	}
	redirect := creds.RedirectURI
	if redirect == "" {
		redirect = strings.TrimSpace(amoRedirectURI)
	}
	body := map[string]string{
		"client_id":     creds.ClientID,
		"client_secret": creds.ClientSecret,
		"grant_type":    "refresh_token",
		"refresh_token": tok.RefreshToken,
		"redirect_uri":  redirect,
	}
	apiURL := strings.TrimRight(creds.AccountDomain, "/") + "/oauth2/access_token"
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("amoCRM token refresh %d: %s", resp.StatusCode, truncateString(string(b), 500))
	}
	var tr amoTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}
	expiresAt := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return saveCustomerAmoTokens(ctx, customerID, amoUserTokens{
		CustomerID:    customerID,
		AccountDomain: strings.TrimRight(creds.AccountDomain, "/"),
		AccessToken:   tr.AccessToken,
		RefreshToken:  tr.RefreshToken,
		ExpiresAt:     expiresAt,
	})
}

func exchangeAmoAuthCodeForCustomer(ctx context.Context, customerID string, code string) error {
	if db == nil {
		return fmt.Errorf("DB not connected")
	}
	if !isAmoConfigured() {
		return fmt.Errorf("amoCRM не сконфигурирован (env)")
	}
	creds, err := loadCustomerAmoCredentials(ctx, customerID)
	if err != nil {
		return err
	}
	redirect := creds.RedirectURI
	if redirect == "" {
		redirect = strings.TrimSpace(amoRedirectURI)
	}
	body := map[string]string{
		"client_id":     creds.ClientID,
		"client_secret": creds.ClientSecret,
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  redirect,
	}
	apiURL := strings.TrimRight(creds.AccountDomain, "/") + "/oauth2/access_token"
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("amoCRM token exchange %d: %s", resp.StatusCode, truncateString(string(b), 500))
	}
	var tr amoTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}
	expiresAt := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return saveCustomerAmoTokens(ctx, customerID, amoUserTokens{
		CustomerID:    customerID,
		AccountDomain: strings.TrimRight(creds.AccountDomain, "/"),
		AccessToken:   tr.AccessToken,
		RefreshToken:  tr.RefreshToken,
		ExpiresAt:     expiresAt,
	})
}

func amoAPIRequestForCustomer(ctx context.Context, customerID string, method string, path string, body io.Reader) (*http.Response, error) {
	token, err := ensureCustomerAmoAccessToken(ctx, customerID)
	if err != nil {
		return nil, err
	}
	tok, err := loadCustomerAmoTokens(ctx, customerID)
	if err != nil || strings.TrimSpace(tok.AccountDomain) == "" {
		return nil, fmt.Errorf("amoCRM account domain not set")
	}
	url := strings.TrimRight(tok.AccountDomain, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	httpClient := &http.Client{Timeout: 20 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		if err := refreshCustomerAmoToken(ctx, customerID); err != nil {
			return nil, err
		}
		token, err = ensureCustomerAmoAccessToken(ctx, customerID)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = httpClient.Do(req)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// Утилита безопасного экранирования для URL-параметров
func urlQueryEscape(s string) string {
	r := strings.NewReplacer(
		" ", "%20",
		"!", "%21",
		"#", "%23",
		"$", "%24",
		"&", "%26",
		"'", "%27",
		"(", "%28",
		")", "%29",
		"*", "%2A",
		"+", "%2B",
		",", "%2C",
		"/", "%2F",
		":", "%3A",
		";", "%3B",
		"=", "%3D",
		"?", "%3F",
		"@", "%40",
		"[", "%5B",
		"]", "%5D",
	)
	return r.Replace(s)
}
