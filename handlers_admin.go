package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// простая сессия в памяти: token -> adminEmail
var adminSessions = map[string]string{}

func handleAdminLogin(w http.ResponseWriter, r *http.Request) {
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

	// вариант 1: использовать bootstrap ADMIN_EMAIL/ADMIN_PASSWORD
	if adminEmail != "" && adminPassword != "" {
		if safeCompareStrings(strings.ToLower(email), strings.ToLower(adminEmail)) && safeCompareStrings(password, adminPassword) {
			issueAdminSession(w, email)
			sendJSONResponse(w, APIResponse{Status: "success"})
			return
		}
	}

	// вариант 2: искать в таблице admin_users (email/password_hash)
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var hash string
	row := db.QueryRow("select password_hash from admin_users where lower(email)=lower($1) and deleted_at is null", email)
	if err := row.Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		log.Println("admin login scan err:", err)
		sendJSONError(w, "server error", http.StatusInternalServerError)
		return
	}
	if !checkPasswordHash(hash, password) {
		sendJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	issueAdminSession(w, email)
	sendJSONResponse(w, APIResponse{Status: "success"})
}

func issueAdminSession(w http.ResponseWriter, email string) {
	token, err := generateSessionToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	adminSessions[token] = email
	cookie := &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	}
	http.SetCookie(w, cookie)
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("admin_session")
	if err == nil && c.Value != "" {
		delete(adminSessions, c.Value)
		c.Expires = time.Unix(0, 0)
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("admin_session")
		if err != nil || c.Value == "" || adminSessions[c.Value] == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- Customers CRUD (минимум) ---

type CustomerDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Company   string `json:"company"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func handleAdminCustomersList(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	rows, err := db.Query(`select id, email, coalesce(name,''), coalesce(company,''), status, created_at from customers where deleted_at is null order by created_at desc limit 500`)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var out []CustomerDTO
	for rows.Next() {
		var c CustomerDTO
		var created time.Time
		if err := rows.Scan(&c.ID, &c.Email, &c.Name, &c.Company, &c.Status, &created); err != nil {
			sendJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		c.CreatedAt = created.Format(time.RFC3339)
		out = append(out, c)
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: out})
}

func handleAdminCustomersCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Company  string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.Email == "" {
		sendJSONError(w, "email required", http.StatusBadRequest)
		return
	}
	// автогенерация пароля, если не передан
	if strings.TrimSpace(payload.Password) == "" {
		if randPass, err := generateSessionToken(); err == nil { // 64 hex
			// укоротим до 16 символов для удобства
			payload.Password = randPass[:16]
		} else {
			payload.Password = "ChangeMe123!"
		}
	}
	hash, err := hashPassword(payload.Password)
	if err != nil {
		sendJSONError(w, "hash error", http.StatusInternalServerError)
		return
	}
	_, err = db.Exec(`insert into customers(id, email, password_hash, name, company) values(gen_random_uuid(), $1, $2, $3, $4)`,
		payload.Email, hash, payload.Name, payload.Company,
	)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: map[string]string{"generatedPassword": payload.Password}})
}

func handleAdminCustomersUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		ID       string  `json:"id"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
		Name     *string `json:"name"`
		Company  *string `json:"company"`
		Status   *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.ID == "" {
		sendJSONError(w, "id required", http.StatusBadRequest)
		return
	}

	// динамическое обновление с корректной нумерацией плейсхолдеров
	setParts := []string{}
	args := []interface{}{}
	idx := 0
	if payload.Email != nil {
		idx++
		setParts = append(setParts, fmt.Sprintf("email=$%d", idx))
		args = append(args, *payload.Email)
	}
	if payload.Name != nil {
		idx++
		setParts = append(setParts, fmt.Sprintf("name=$%d", idx))
		args = append(args, *payload.Name)
	}
	if payload.Company != nil {
		idx++
		setParts = append(setParts, fmt.Sprintf("company=$%d", idx))
		args = append(args, *payload.Company)
	}
	if payload.Status != nil {
		idx++
		setParts = append(setParts, fmt.Sprintf("status=$%d", idx))
		args = append(args, *payload.Status)
	}
	if payload.Password != nil && *payload.Password != "" {
		hash, _ := hashPassword(*payload.Password)
		idx++
		setParts = append(setParts, fmt.Sprintf("password_hash=$%d", idx))
		args = append(args, hash)
	}
	// всегда обновляем updated_at
	setParts = append(setParts, "updated_at=now()")
	if len(setParts) == 1 { // ничего не изменилось, кроме updated_at
		// всё равно выполнем только обновление updated_at
	}
	idx++
	wherePlaceholder := fmt.Sprintf("$%d", idx)
	args = append(args, payload.ID)
	q := "update customers set " + strings.Join(setParts, ", ") + " where id=" + wherePlaceholder
	if _, err := db.Exec(q, args...); err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

func handleAdminCustomersDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.ID == "" {
		sendJSONError(w, "id required", http.StatusBadRequest)
		return
	}
	if _, err := db.Exec(`update customers set deleted_at=now() where id=$1 and deleted_at is null`, payload.ID); err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

// удалён неиспользуемый helper itoa

// --- Subscriptions ---
func handleAdminSubscriptionsCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var p struct {
		CustomerID  string `json:"customer_id"`
		QuotaTotal  int    `json:"quota_total"`
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
		AutoRenew   *bool  `json:"auto_renew"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		sendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if p.CustomerID == "" || p.QuotaTotal <= 0 || p.PeriodStart == "" || p.PeriodEnd == "" {
		sendJSONError(w, "fields required", http.StatusBadRequest)
		return
	}
	// валидация дат: формат, end>=start, end не в прошлом
	start, err1 := time.Parse("2006-01-02", p.PeriodStart)
	end, err2 := time.Parse("2006-01-02", p.PeriodEnd)
	if err1 != nil || err2 != nil {
		sendJSONError(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	if end.Before(start) {
		sendJSONError(w, "period_end must be >= period_start", http.StatusBadRequest)
		return
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if end.Before(today) {
		sendJSONError(w, "period_end must not be in the past", http.StatusBadRequest)
		return
	}
	auto := true
	if p.AutoRenew != nil {
		auto = *p.AutoRenew
	}
	_, err := db.Exec(`insert into subscriptions(id, customer_id, status, period_start, period_end, quota_total, auto_renew) values(gen_random_uuid(), $1, 'active', $2, $3, $4, $5)`, p.CustomerID, start.Format("2006-01-02"), end.Format("2006-01-02"), p.QuotaTotal, auto)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success"})
}

// Сгенерировать новый пароль для клиента (одноразово вернуть в ответе)
func handleAdminCustomersRegeneratePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		sendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.ID) == "" {
		sendJSONError(w, "id required", http.StatusBadRequest)
		return
	}
	// generate new password
	token, err := generateSessionToken()
	if err != nil {
		sendJSONError(w, "cannot generate password", http.StatusInternalServerError)
		return
	}
	newPass := token[:16]
	hash, err := hashPassword(newPass)
	if err != nil {
		sendJSONError(w, "hash error", http.StatusInternalServerError)
		return
	}
	res, err := db.Exec(`update customers set password_hash=$1, updated_at=now() where id=$2 and deleted_at is null`, hash, payload.ID)
	if err != nil {
		sendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		sendJSONError(w, "customer not found", http.StatusNotFound)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success", Data: map[string]string{"generatedPassword": newPass}})
}
