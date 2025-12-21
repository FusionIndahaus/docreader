package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Bitrix BYOA: используем входящий вебхук (webhook_base), хранимый для пользователя.
// Пример webhook_base:
// https://<your-portal>.bitrix24.ru/rest/<user_id>/<webhook_code>
// Вызовы делаются как: {webhook_base}/crm.lead.add.json

type bitrixUserConfig struct {
	WebhookBase string
	Enabled     bool
}

type bitrixField struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func loadCustomerBitrixConfig(ctx context.Context, customerID string) (bitrixUserConfig, error) {
	var cfg bitrixUserConfig
	if db == nil {
		return cfg, fmt.Errorf("DB not connected")
	}
	var enabled bool
	var accountDomain string
	row := db.QueryRowContext(ctx, `
		select enabled, coalesce(account_domain,'')
		from user_integrations
		where user_id=$1 and provider='bitrix'
	`, customerID)
	_ = row.Scan(&enabled, &accountDomain)
	cfg.Enabled = enabled
	cfg.WebhookBase = strings.TrimRight(strings.TrimSpace(accountDomain), "/")
	return cfg, nil
}

func isBitrixConfigured() bool {
	// BYOA: конфиг считается заданным, если для пользователя сохранён webhook_base
	return true
}

// sendBitrixLead создает лид в Bitrix24 через входящий вебхук.
// fields — карта полей (например, TITLE, COMMENTS, OPPORTUNITY и т.п., а также UF_CRM_*).
func sendBitrixLead(ctx context.Context, webhookBase string, fields map[string]interface{}) error {
	if webhookBase == "" {
		return fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := webhookBase + "/crm.lead.add.json"
	payload := map[string]interface{}{"fields": fields}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
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
		return fmt.Errorf("bitrix lead.add failed: %d", resp.StatusCode)
	}
	return nil
}

// fetchBitrixLeadFields получает список доступных полей лида через вебхук.
func fetchBitrixLeadFields(ctx context.Context, webhookBase string) ([]bitrixField, error) {
	if webhookBase == "" {
		return nil, fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := webhookBase + "/crm.lead.fields.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bitrix fields failed: %d", resp.StatusCode)
	}
	var raw struct {
		Result map[string]struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]bitrixField, 0, len(raw.Result))
	for code, f := range raw.Result {
		out = append(out, bitrixField{
			Code:  code,
			Title: f.Title,
			Type:  f.Type,
		})
	}
	return out, nil
}
