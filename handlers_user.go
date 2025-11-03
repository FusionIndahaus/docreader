package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// сессии пользователей: token -> customerEmail
var userSessions = map[string]string{}

func handleUserLogin(w http.ResponseWriter, r *http.Request) {
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
		sendJSONError(w, "email and password required", http.StatusBadRequest)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var hash string
	row := db.QueryRow("select password_hash from customers where deleted_at is null and lower(email)=lower($1)", email)
	if err := row.Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}
	if !checkPasswordHash(hash, password) {
		sendJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := generateSessionToken()
	if err != nil {
		sendJSONError(w, "session error", http.StatusInternalServerError)
		return
	}
	userSessions[token] = email
	cookie := &http.Cookie{
		Name:     "user_session",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	}
	http.SetCookie(w, cookie)
	sendJSONResponse(w, APIResponse{Status: "success"})
}

func handleUserLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err == nil && c.Value != "" {
		delete(userSessions, c.Value)
		c.Expires = time.Unix(0, 0)
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

func requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("user_session")
		if err != nil || c.Value == "" || userSessions[c.Value] == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleUserProfile godoc
// @Summary Получить профиль пользователя
// @Description Возвращает информацию о текущем пользователе
// @Tags User
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/profile [get]
func handleUserProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		handleUserProfileGet(w, r)
	} else if r.Method == http.MethodPut {
		handleUserProfileUpdate(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func handleUserProfileGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var customer struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Company string `json:"company"`
		Status  string `json:"status"`
	}

	row := db.QueryRow(`
		SELECT email, name, company, status 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)

	if err := row.Scan(&customer.Email, &customer.Name, &customer.Company, &customer.Status); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   customer,
	})
}

func handleUserProfileUpdate(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var updateData struct {
		Name    string `json:"name"`
		Company string `json:"company"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`
		UPDATE customers 
		SET name = $1, company = $2, updated_at = now()
		WHERE deleted_at IS NULL AND lower(email) = lower($3)
	`, updateData.Name, updateData.Company, email)

	if err != nil {
		sendJSONError(w, "update failed", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Profile updated successfully",
	})
}

// handleUserSettings godoc
// @Summary Получить/обновить настройки пользователя
// @Description Возвращает или обновляет настройки пользователя
// @Tags User
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/settings [get]
func handleUserSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		handleUserSettingsGet(w, r)
	} else if r.Method == http.MethodPut {
		handleUserSettingsUpdate(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func handleUserSettingsGet(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var metadata string
	row := db.QueryRow(`
		SELECT metadata 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)

	if err := row.Scan(&metadata); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	// Парсим метаданные
	var settings map[string]interface{}
	if metadata == "" || metadata == "{}" {
		settings = map[string]interface{}{
			"output_formats": []string{"csv", "xlsx", "json"},
		}
	} else {
		if err := json.Unmarshal([]byte(metadata), &settings); err != nil {
			settings = map[string]interface{}{
				"output_formats": []string{"csv", "xlsx", "json"},
			}
		}
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   settings,
	})
}

func handleUserSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var updateData struct {
		OutputFormats []string `json:"output_formats"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация форматов
	validFormats := map[string]bool{"csv": true, "xlsx": true, "json": true}
	for _, format := range updateData.OutputFormats {
		if !validFormats[format] {
			sendJSONError(w, "invalid format", http.StatusBadRequest)
			return
		}
	}

	// Проверяем, что хотя бы один формат выбран
	if len(updateData.OutputFormats) == 0 {
		sendJSONError(w, "at least one format must be selected", http.StatusBadRequest)
		return
	}

	// Получаем текущие метаданные
	var currentMetadata string
	row := db.QueryRow(`
		SELECT metadata 
		FROM customers 
		WHERE deleted_at IS NULL AND lower(email) = lower($1)
	`, email)

	if err := row.Scan(&currentMetadata); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	// Парсим текущие метаданные
	var settings map[string]interface{}
	if currentMetadata == "" || currentMetadata == "{}" {
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal([]byte(currentMetadata), &settings); err != nil {
			settings = make(map[string]interface{})
		}
	}

	// Обновляем настройки форматов
	settings["output_formats"] = updateData.OutputFormats

	// Сериализуем обратно в JSON
	metadataJSON, err := json.Marshal(settings)
	if err != nil {
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	// Обновляем в базе данных
	_, err = db.Exec(`
		UPDATE customers 
		SET metadata = $1, updated_at = now()
		WHERE deleted_at IS NULL AND lower(email) = lower($2)
	`, string(metadataJSON), email)

	if err != nil {
		sendJSONError(w, "update failed", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Settings updated successfully",
	})
}

// handleUserChangePassword godoc
// @Summary Изменить пароль пользователя
// @Description Изменяет пароль пользователя
// @Tags User
// @Accept json
// @Produce json
// @Param body body object true "Данные для смены пароля"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/change-password [post]
func handleUserChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var changePasswordData struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&changePasswordData); err != nil {
		sendJSONError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if changePasswordData.CurrentPassword == "" || changePasswordData.NewPassword == "" {
		sendJSONError(w, "current_password and new_password required", http.StatusBadRequest)
		return
	}

	// Проверка длины нового пароля
	if len(changePasswordData.NewPassword) < 6 {
		sendJSONError(w, "new password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	// Проверяем текущий пароль
	var hash string
	row := db.QueryRow("SELECT password_hash FROM customers WHERE deleted_at IS NULL AND lower(email)=lower($1)", email)
	if err := row.Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "user not found", http.StatusNotFound)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	if !checkPasswordHash(hash, changePasswordData.CurrentPassword) {
		sendJSONError(w, "invalid current password", http.StatusUnauthorized)
		return
	}

	// Хешируем новый пароль
	newHash, err := hashPassword(changePasswordData.NewPassword)
	if err != nil {
		sendJSONError(w, "password hashing error", http.StatusInternalServerError)
		return
	}

	// Обновляем пароль
	_, err = db.Exec(`
		UPDATE customers 
		SET password_hash = $1, updated_at = now()
		WHERE deleted_at IS NULL AND lower(email) = lower($2)
	`, newHash, email)

	if err != nil {
		sendJSONError(w, "update failed", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Password changed successfully",
	})
}

// handleUserSubscription godoc
// @Summary Получить информацию о подписке
// @Description Возвращает информацию о текущей подписке пользователя
// @Tags User
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/subscription [get]
func handleUserSubscription(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var subscription struct {
		Status      string `json:"status"`
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
		QuotaTotal  int    `json:"quota_total"`
		UsageCount  int    `json:"usage_count"`
	}

	row := db.QueryRow(`
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
		if err == sql.ErrNoRows {
			sendJSONError(w, "no active subscription", http.StatusNotFound)
			return
		}
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   subscription,
	})
}

// handleUserHistory godoc
// @Summary Получить историю загрузок
// @Description Возвращает историю обработки документов пользователя
// @Tags User
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/history [get]
func handleUserHistory(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	// Получаем историю из глобального массива responses, фильтруя по пользователю
	responsesMutex.RLock()
	userHistory := append([]ProcessingResponse{}, responses...)
	responsesMutex.RUnlock()

	// Ограничиваем количество записей
	if len(userHistory) > 50 {
		userHistory = userHistory[:50]
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   userHistory,
	})
}

// handleUserUsageStats godoc
// @Summary Получить статистику использования
// @Description Возвращает статистику использования квоты пользователя
// @Tags User
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /user/usage-stats [get]
func handleUserUsageStats(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || userSessions[c.Value] == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := userSessions[c.Value]
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}

	var stats struct {
		TotalProcessed   int `json:"total_processed"`
		MonthlyProcessed int `json:"monthly_processed"`
		RemainingQuota   int `json:"remaining_quota"`
	}

	// Общее количество обработанных документов
	row := db.QueryRow(`
		SELECT COALESCE(SUM(ue.amount), 0) as total_processed
		FROM customers c
		JOIN subscriptions s ON c.id = s.customer_id
		LEFT JOIN usage_events ue ON s.id = ue.subscription_id
		WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
		AND lower(c.email) = lower($1)
	`, email)
	row.Scan(&stats.TotalProcessed)

	// За последний месяц
	row = db.QueryRow(`
		SELECT COALESCE(SUM(ue.amount), 0) as monthly_processed
		FROM customers c
		JOIN subscriptions s ON c.id = s.customer_id
		LEFT JOIN usage_events ue ON s.id = ue.subscription_id
		WHERE c.deleted_at IS NULL AND s.deleted_at IS NULL 
		AND lower(c.email) = lower($1)
		AND ue.occurred_at >= NOW() - INTERVAL '1 month'
	`, email)
	row.Scan(&stats.MonthlyProcessed)

	// Оставшаяся квота
	row = db.QueryRow(`
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

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   stats,
	})
}
