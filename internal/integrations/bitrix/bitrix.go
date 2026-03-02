package bitrix

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

type Field struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type UserConfig struct {
	WebhookBase string
	Enabled     bool
}

type userField struct {
	FieldName       string      `json:"FIELD_NAME"`
	EditFormLabel   interface{} `json:"EDIT_FORM_LABEL"`
	ListColumnLabel interface{} `json:"LIST_COLUMN_LABEL"`
	ListFilterLabel interface{} `json:"LIST_FILTER_LABEL"`
}

func extractLabel(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]interface{}:
		if vv, ok := t["ru"]; ok {
			if s, ok2 := vv.(string); ok2 && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		if vv, ok := t["en"]; ok {
			if s, ok2 := vv.(string); ok2 && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		for _, vv := range t {
			if s, ok2 := vv.(string); ok2 && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func fetchLeadUserFieldLabels(ctx context.Context, webhookBase string) (map[string]string, error) {
	if webhookBase == "" {
		return nil, fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.lead.userfield.list.json"
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
		return nil, fmt.Errorf("bitrix lead.userfield.list failed: %d", resp.StatusCode)
	}
	var raw struct {
		Result []userField `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw.Result))
	for _, uf := range raw.Result {
		label := extractLabel(uf.EditFormLabel)
		if label == "" {
			label = extractLabel(uf.ListColumnLabel)
		}
		if label == "" {
			label = extractLabel(uf.ListFilterLabel)
		}
		if label != "" && uf.FieldName != "" {
			out[uf.FieldName] = label
		}
	}
	return out, nil
}

func fetchDealUserFieldLabels(ctx context.Context, webhookBase string) (map[string]string, error) {
	if webhookBase == "" {
		return nil, fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.deal.userfield.list.json"
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
		return nil, fmt.Errorf("bitrix deal.userfield.list failed: %d", resp.StatusCode)
	}
	var raw struct {
		Result []userField `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw.Result))
	for _, uf := range raw.Result {
		label := extractLabel(uf.EditFormLabel)
		if label == "" {
			label = extractLabel(uf.ListColumnLabel)
		}
		if label == "" {
			label = extractLabel(uf.ListFilterLabel)
		}
		if label != "" && uf.FieldName != "" {
			out[uf.FieldName] = label
		}
	}
	return out, nil
}

func FetchLeadFields(ctx context.Context, webhookBase string) ([]Field, error) {
	if webhookBase == "" {
		return nil, fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.lead.fields.json"
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
			Title       string      `json:"title"`
			Type        string      `json:"type"`
			FormLabel   interface{} `json:"formLabel"`
			ListLabel   interface{} `json:"listLabel"`
			FilterLabel interface{} `json:"filterLabel"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	labels, _ := fetchLeadUserFieldLabels(ctx, webhookBase)
	out := make([]Field, 0, len(raw.Result))
	for code, f := range raw.Result {
		bf := Field{
			Code:  code,
			Title: f.Title,
			Type:  f.Type,
		}
		if lbl := extractLabel(f.FormLabel); lbl != "" {
			bf.Title = lbl
		} else if lbl := extractLabel(f.ListLabel); lbl != "" {
			bf.Title = lbl
		} else if lbl := extractLabel(f.FilterLabel); lbl != "" {
			bf.Title = lbl
		} else if strings.HasPrefix(code, "UF_CRM_") {
			if lbl, ok := labels[code]; ok && lbl != "" {
				bf.Title = lbl
			}
		}
		out = append(out, bf)
	}
	return out, nil
}

func FetchDealFields(ctx context.Context, webhookBase string) ([]Field, error) {
	if webhookBase == "" {
		return nil, fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.deal.fields.json"
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
		return nil, fmt.Errorf("bitrix deal fields failed: %d", resp.StatusCode)
	}
	var raw struct {
		Result map[string]struct {
			Title       string      `json:"title"`
			Type        string      `json:"type"`
			FormLabel   interface{} `json:"formLabel"`
			ListLabel   interface{} `json:"listLabel"`
			FilterLabel interface{} `json:"filterLabel"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	labels, _ := fetchDealUserFieldLabels(ctx, webhookBase)
	out := make([]Field, 0, len(raw.Result))
	for code, f := range raw.Result {
		bf := Field{
			Code:  code,
			Title: f.Title,
			Type:  f.Type,
		}
		if lbl := extractLabel(f.FormLabel); lbl != "" {
			bf.Title = lbl
		} else if lbl := extractLabel(f.ListLabel); lbl != "" {
			bf.Title = lbl
		} else if lbl := extractLabel(f.FilterLabel); lbl != "" {
			bf.Title = lbl
		} else if strings.HasPrefix(code, "UF_CRM_") {
			if lbl, ok := labels[code]; ok && lbl != "" {
				bf.Title = lbl
			}
		}
		out = append(out, bf)
	}
	return out, nil
}

// LoadUserConfig loads Bitrix webhook base and enabled flag for a specific user.
func LoadUserConfig(ctx context.Context, db *sql.DB, customerID string) (UserConfig, error) {
	var cfg UserConfig
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

func SendLead(ctx context.Context, webhookBase string, fields map[string]interface{}) error {
	if webhookBase == "" {
		return fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.lead.add.json"
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

func SendDeal(ctx context.Context, webhookBase string, fields map[string]interface{}) error {
	if webhookBase == "" {
		return fmt.Errorf("bitrix webhook base is empty")
	}
	apiURL := strings.TrimRight(webhookBase, "/") + "/crm.deal.add.json"
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
		return fmt.Errorf("bitrix deal.add failed: %d", resp.StatusCode)
	}
	return nil
}
