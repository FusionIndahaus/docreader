package onec

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// UserConfig хранит пользовательские настройки интеграции 1С.
type UserConfig struct {
	BaseURL      string // account_domain из user_integrations
	Enabled      bool
	EndpointPath string // из config.endpoint_path
}

// Credentials хранит секреты/метод аутентификации для 1С.
type Credentials struct {
	AuthType string // none|basic|bearer
	Username string
	Password string
	Token    string
}

// LoadUserConfig загружает включенность, базовый URL и endpoint_path для конкретного пользователя.
func LoadUserConfig(ctx context.Context, db *sql.DB, customerID string) (UserConfig, error) {
	var cfg UserConfig
	if db == nil {
		return cfg, fmt.Errorf("DB not connected")
	}
	var enabled bool
	var accountDomain string
	var confJSON string
	row := db.QueryRowContext(ctx, `
		select enabled, coalesce(account_domain,''), coalesce(config::text,'{}')
		from user_integrations
		where user_id=$1 and provider='1c'
	`, customerID)
	_ = row.Scan(&enabled, &accountDomain, &confJSON)
	cfg.Enabled = enabled
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(accountDomain), "/")
	if confJSON != "" {
		var conf map[string]interface{}
		_ = json.Unmarshal([]byte(confJSON), &conf)
		if v, ok := conf["endpoint_path"].(string); ok {
			cfg.EndpointPath = v
		}
	}
	return cfg, nil
}

// LoadCredentials загружает метод аутентификации и секреты для 1С.
func LoadCredentials(ctx context.Context, db *sql.DB, customerID string) (Credentials, error) {
	var c Credentials
	if db == nil {
		return c, fmt.Errorf("DB not connected")
	}
	var credJSON string
	row := db.QueryRowContext(ctx, `
		select coalesce(credentials::text,'{}')
		from user_integrations
		where user_id=$1 and provider='1c'
	`, customerID)
	_ = row.Scan(&credJSON)
	if credJSON != "" {
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(credJSON), &m)
		if v, ok := m["auth_type"].(string); ok {
			c.AuthType = strings.ToLower(strings.TrimSpace(v))
		}
		if v, ok := m["username"].(string); ok {
			c.Username = v
		}
		if v, ok := m["password"].(string); ok {
			c.Password = v
		}
		if v, ok := m["token"].(string); ok {
			c.Token = v
		}
	}
	if c.AuthType == "" {
		c.AuthType = "none"
	}
	return c, nil
}

// SendJSON отправляет полезную нагрузку в 1С по заданным настройкам и кредам.
func SendJSON(ctx context.Context, cfg UserConfig, cred Credentials, payload map[string]interface{}) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("1C base URL is empty")
	}
	path := strings.TrimSpace(cfg.EndpointPath)
	url := cfg.BaseURL
	if path != "" {
		url = strings.TrimRight(cfg.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	switch cred.AuthType {
	case "basic":
		if cred.Username != "" {
			req.SetBasicAuth(cred.Username, cred.Password)
		}
	case "bearer":
		if cred.Token != "" {
			req.Header.Set("Authorization", "Bearer "+cred.Token)
		}
	default:
		// none
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("1C send failed: %d", resp.StatusCode)
	}
	return nil
}
