package handlers

import (
	"database/sql"
	amocrmpkg "document-ai/internal/integrations/amocrm"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type UserDeps struct {
	DB                   *sql.DB
	RequireUser          func(http.HandlerFunc) http.HandlerFunc
	RequireUserOrAdmin   func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse     func(http.ResponseWriter, interface{})
	SendJSONError        func(http.ResponseWriter, string, int)
	GetUserEmail         func(string) string
	SetUserSession       func(string, string)
	DeleteUserSession    func(string)
	GenerateSessionToken func() (string, error)
	HashPassword         func(string) (string, error)
	CheckPasswordHash    func(string, string) bool
	GetCustomerIDByEmail func(string) (string, error)
	GetUserHistory       func(email string) interface{} // returns history slice for user
}

func RegisterUserRoutes(mux *http.ServeMux, d UserDeps) {
	mux.HandleFunc("/user/login", handleUserLogin(d))
	mux.HandleFunc("/user/logout", handleUserLogout(d))
	mux.HandleFunc("/user/profile", d.RequireUser(handleUserProfile(d)))
	mux.HandleFunc("/user/change-password", d.RequireUser(handleUserChangePassword(d)))
	mux.HandleFunc("/user/settings", d.RequireUser(handleUserSettings(d)))
	mux.HandleFunc("/user/subscription", d.RequireUser(handleUserSubscription(d)))
	mux.HandleFunc("/user/history", d.RequireUser(handleUserHistory(d)))
	mux.HandleFunc("/user/usage-stats", d.RequireUser(handleUserUsageStats(d)))
}

func handleUserLogin(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		if email == "" || password == "" {
			d.SendJSONError(w, "email and password required", http.StatusBadRequest)
			return
		}
		if d.DB == nil {
			d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
			return
		}
		var hash string
		row := d.DB.QueryRow("select password_hash from customers where deleted_at is null and lower(email)=lower($1)", email)
		if err := row.Scan(&hash); err != nil {
			d.SendJSONError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if !d.CheckPasswordHash(hash, password) {
			d.SendJSONError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		token, err := d.GenerateSessionToken()
		if err != nil {
			d.SendJSONError(w, "session error", http.StatusInternalServerError)
			return
		}
		d.SetUserSession(token, email)
		cookie := &http.Cookie{
			Name:     "user_session",
			Value:    token,
			HttpOnly: true,
			Path:     "/",
			Secure:   cookieSecure,
			SameSite: cookieSameSite,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
		}
		http.SetCookie(w, cookie)
		d.SendJSONResponse(w, APIResponse{Status: "success"})
	}
}

func handleUserLogout(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("user_session")
		if err == nil && c.Value != "" {
			d.DeleteUserSession(c.Value)
			c.Expires = time.Unix(0, 0)
			c.MaxAge = -1
			http.SetCookie(w, c)
		}
		d.SendJSONResponse(w, APIResponse{Status: "success"})
	}
}

func handleUserProfile(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserProfileGet(w, r, d)
			return
		}
		if r.Method == http.MethodPut {
			handleUserProfileUpdate(w, r, d)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleUserProfileGet(w http.ResponseWriter, r *http.Request, d UserDeps) {
	email := getRequestUserEmail(r, d.GetUserEmail)
	if email == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var customer struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Company string `json:"company"`
		Status  string `json:"status"`
	}
	row := d.DB.QueryRow(`
		SELECT email, name, company, status 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)
	if err := row.Scan(&customer.Email, &customer.Name, &customer.Company, &customer.Status); err != nil {
		d.SendJSONError(w, "user not found", http.StatusNotFound)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: customer})
}

func handleUserProfileUpdate(w http.ResponseWriter, r *http.Request, d UserDeps) {
	email := getRequestUserEmail(r, d.GetUserEmail)
	if email == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var updateData struct {
		Name    string `json:"name"`
		Company string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	_, err := d.DB.Exec(`
		UPDATE customers 
		SET name = $1, company = $2, updated_at = now()
		WHERE deleted_at IS NULL AND lower(email) = lower($3)
	`, updateData.Name, updateData.Company, email)
	if err != nil {
		d.SendJSONError(w, "update failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Message: "Profile updated successfully"})
}

func handleUserSettings(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleUserSettingsGet(w, r, d)
			return
		}
		if r.Method == http.MethodPut {
			handleUserSettingsUpdate(w, r, d)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleUserSettingsGet(w http.ResponseWriter, r *http.Request, d UserDeps) {
	email := getRequestUserEmail(r, d.GetUserEmail)
	if email == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var metadata string
	row := d.DB.QueryRow(`
		SELECT metadata 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)
	if err := row.Scan(&metadata); err != nil {
		d.SendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}
	var settings map[string]interface{}
	if metadata == "" || metadata == "{}" {
		settings = map[string]interface{}{}
	} else {
		if err := json.Unmarshal([]byte(metadata), &settings); err != nil {
			settings = map[string]interface{}{}
		}
	}
	if _, ok := settings["output_formats"]; !ok {
		settings["output_formats"] = []string{"csv", "xlsx", "json"}
	}
	defaultDest := map[string]bool{"json": true, "amocrm": false, "bitrix": false, "1c": false}
	if v, ok := settings["destinations"]; !ok {
		settings["destinations"] = defaultDest
	} else {
		if m, ok2 := v.(map[string]interface{}); ok2 {
			out := map[string]bool{}
			for k, def := range defaultDest {
				if raw, ok3 := m[k]; ok3 {
					if b, ok4 := raw.(bool); ok4 {
						out[k] = b
						continue
					}
				}
				out[k] = def
			}
			settings["destinations"] = out
		} else {
			settings["destinations"] = defaultDest
		}
	}
	var amoConnected bool
	if amocrmpkg.IsConfigured() {
		if customerID, err := d.GetCustomerIDByEmail(email); err == nil && customerID != "" {
			if tok, err := amocrmpkg.LoadCustomerTokens(r.Context(), d.DB, customerID); err == nil && tok.AccessToken != "" {
				amoConnected = true
			}
		}
	}
	settings["amocrm_connected"] = amoConnected
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: settings})
}

func handleUserSettingsUpdate(w http.ResponseWriter, r *http.Request, d UserDeps) {
	email := getRequestUserEmail(r, d.GetUserEmail)
	if email == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var updateData struct {
		OutputFormats []string        `json:"output_formats"`
		Destinations  map[string]bool `json:"destinations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	validFormats := map[string]bool{"csv": true, "xlsx": true, "json": true}
	for _, format := range updateData.OutputFormats {
		if !validFormats[format] {
			d.SendJSONError(w, "invalid format", http.StatusBadRequest)
			return
		}
	}
	if len(updateData.OutputFormats) == 0 {
		d.SendJSONError(w, "at least one format must be selected", http.StatusBadRequest)
		return
	}
	var currentMetadata string
	row := d.DB.QueryRow(`
		SELECT metadata 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)
	if err := row.Scan(&currentMetadata); err != nil {
		d.SendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}
	var settings map[string]interface{}
	if currentMetadata == "" || currentMetadata == "{}" {
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal([]byte(currentMetadata), &settings); err != nil {
			settings = make(map[string]interface{})
		}
	}
	settings["output_formats"] = updateData.OutputFormats
	if updateData.Destinations != nil {
		validKeys := map[string]bool{"json": true, "amocrm": true, "bitrix": true, "1c": true}
		clean := map[string]bool{}
		for k, v := range updateData.Destinations {
			if validKeys[k] {
				clean[k] = v
			}
		}
		settings["destinations"] = clean
	}
	metadataJSON, err := json.Marshal(settings)
	if err != nil {
		d.SendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}
	_, err = d.DB.Exec(`
		UPDATE customers 
		SET metadata = $1, updated_at = now()
		WHERE deleted_at IS NULL AND lower(email) = lower($2)
	`, string(metadataJSON), email)
	if err != nil {
		d.SendJSONError(w, "update failed", http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Message: "Settings updated successfully"})
}

func handleUserChangePassword(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		email := getRequestUserEmail(r, d.GetUserEmail)
		if email == "" {
			d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if d.DB == nil {
			d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
			return
		}
		var changePasswordData struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&changePasswordData); err != nil {
			d.SendJSONError(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if changePasswordData.CurrentPassword == "" || changePasswordData.NewPassword == "" {
			d.SendJSONError(w, "current_password and new_password required", http.StatusBadRequest)
			return
		}
		if len(changePasswordData.NewPassword) < 6 {
			d.SendJSONError(w, "new password must be at least 6 characters", http.StatusBadRequest)
			return
		}
		var hash string
		row := d.DB.QueryRow("SELECT password_hash FROM customers WHERE deleted_at IS NULL AND lower(email)=lower($1)", email)
		if err := row.Scan(&hash); err != nil {
			d.SendJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		if !d.CheckPasswordHash(hash, changePasswordData.CurrentPassword) {
			d.SendJSONError(w, "invalid current password", http.StatusUnauthorized)
			return
		}
		newHash, err := d.HashPassword(changePasswordData.NewPassword)
		if err != nil {
			d.SendJSONError(w, "password hashing error", http.StatusInternalServerError)
			return
		}
		_, err = d.DB.Exec(`
			UPDATE customers 
			SET password_hash = $1, updated_at = now()
			WHERE deleted_at IS NULL AND lower(email) = lower($2)
		`, newHash, email)
		if err != nil {
			d.SendJSONError(w, "update failed", http.StatusInternalServerError)
			return
		}
		d.SendJSONResponse(w, APIResponse{Status: "success", Message: "Password changed successfully"})
	}
}

func handleUserSubscription(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := getRequestUserEmail(r, d.GetUserEmail)
		if email == "" {
			d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if d.DB == nil {
			d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
			return
		}
		var subscription struct {
			Status      string `json:"status"`
			PeriodStart string `json:"period_start"`
			PeriodEnd   string `json:"period_end"`
			QuotaTotal  int    `json:"quota_total"`
			UsageCount  int    `json:"usage_count"`
		}
		row := d.DB.QueryRow(`
			SELECT s.status, s.period_start, s.period_end, s.quota_total,
			       COALESCE(SUM(ue.amount), 0) as usage_count
			FROM customers c
			JOIN subscriptions s ON c.id = s.customer_id
			LEFT JOIN usage_events ue ON s.id = ue.subscription_id
			WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
			AND lower(c.email) = lower($1)
			GROUP BY s.id, s.status, s.period_start, s.period_end, s.quota_total
			ORDER BY s.created_at DESC
			LIMIT 1
		`, email)
		if err := row.Scan(&subscription.Status, &subscription.PeriodStart, &subscription.PeriodEnd, &subscription.QuotaTotal, &subscription.UsageCount); err != nil {
			d.SendJSONError(w, "no active subscription", http.StatusNotFound)
			return
		}
		d.SendJSONResponse(w, APIResponse{Status: "success", Data: subscription})
	}
}

func handleUserHistory(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := getRequestUserEmail(r, d.GetUserEmail)
		if email == "" {
			d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		history := d.GetUserHistory(email)
		d.SendJSONResponse(w, APIResponse{Status: "success", Data: history})
	}
}

func handleUserUsageStats(d UserDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := getRequestUserEmail(r, d.GetUserEmail)
		if email == "" {
			d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if d.DB == nil {
			d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
			return
		}
		var stats struct {
			TotalProcessed   int `json:"total_processed"`
			MonthlyProcessed int `json:"monthly_processed"`
			RemainingQuota   int `json:"remaining_quota"`
		}
		row := d.DB.QueryRow(`
			SELECT COALESCE(SUM(ue.amount), 0) as total_processed
			FROM customers c
			JOIN subscriptions s ON c.id = s.customer_id
			LEFT JOIN usage_events ue ON s.id = ue.subscription_id
			WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
			AND lower(c.email) = lower($1)
		`, email)
		row.Scan(&stats.TotalProcessed)
		row = d.DB.QueryRow(`
			SELECT COALESCE(SUM(ue.amount), 0) as monthly_processed
			FROM customers c
			JOIN subscriptions s ON c.id = s.customer_id
			LEFT JOIN usage_events ue ON s.id = ue.subscription_id
			WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
			AND lower(c.email) = lower($1)
			AND ue.occurred_at >= NOW() - INTERVAL '1 month'
		`, email)
		row.Scan(&stats.MonthlyProcessed)
		row = d.DB.QueryRow(`
			SELECT s.quota_total - COALESCE(SUM(ue.amount), 0) as remaining_quota
			FROM customers c
			JOIN subscriptions s ON c.id = s.customer_id
			LEFT JOIN usage_events ue ON s.id = ue.subscription_id
			WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
			AND lower(c.email) = lower($1)
			GROUP BY s.quota_total
			ORDER BY s.created_at DESC
			LIMIT 1
		`, email)
		row.Scan(&stats.RemainingQuota)
		if stats.RemainingQuota < 0 {
			stats.RemainingQuota = 0
		}
		d.SendJSONResponse(w, APIResponse{Status: "success", Data: stats})
	}
}
