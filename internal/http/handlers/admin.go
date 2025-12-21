package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AdminDeps struct {
	DB                   *sql.DB
	RequireAdmin         func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse     func(http.ResponseWriter, interface{})
	SendJSONError        func(http.ResponseWriter, string, int)
	GenerateSessionToken func() (string, error)
	HashPassword         func(string) (string, error)
	CheckPasswordHash    func(string, string) bool
	SafeCompareStrings   func(string, string) bool
	AdminEmail           string
	AdminPassword        string
	AdminSessions        map[string]string
}

func RegisterAdminRoutes(mux *http.ServeMux, d AdminDeps) {
	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogin(w, r, d)
	})
	mux.HandleFunc("/admin/logout", func(w http.ResponseWriter, r *http.Request) {
		handleAdminLogout(w, r, d)
	})
	mux.HandleFunc("/admin/customers", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminCustomersList(w, r, d)
	}))
	mux.HandleFunc("/admin/customers/create", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminCustomersCreate(w, r, d)
	}))
	mux.HandleFunc("/admin/customers/update", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminCustomersUpdate(w, r, d)
	}))
	mux.HandleFunc("/admin/customers/delete", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminCustomersDelete(w, r, d)
	}))
	mux.HandleFunc("/admin/customers/regenerate_password", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminCustomersRegeneratePassword(w, r, d)
	}))
	mux.HandleFunc("/admin/subscriptions/create", d.RequireAdmin(func(w http.ResponseWriter, r *http.Request) {
		handleAdminSubscriptionsCreate(w, r, d)
	}))
}

func issueAdminSession(w http.ResponseWriter, email string, d AdminDeps) {
	token, err := d.GenerateSessionToken()
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	d.AdminSessions[token] = email
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

func handleAdminLogin(w http.ResponseWriter, r *http.Request, d AdminDeps) {
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

	if d.AdminEmail != "" && d.AdminPassword != "" {
		if d.SafeCompareStrings(strings.ToLower(email), strings.ToLower(d.AdminEmail)) && d.SafeCompareStrings(password, d.AdminPassword) {
			issueAdminSession(w, email, d)
			d.SendJSONResponse(w, APIResponse{Status: "success"})
			return
		}
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var hash string
	row := d.DB.QueryRow("select password_hash from admin_users where lower(email)=lower($1) and deleted_at is null", email)
	if err := row.Scan(&hash); err != nil {
		d.SendJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if !d.CheckPasswordHash(hash, password) {
		d.SendJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	issueAdminSession(w, email, d)
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	c, err := r.Cookie("admin_session")
	if err == nil && c.Value != "" {
		delete(d.AdminSessions, c.Value)
		c.Expires = time.Unix(0, 0)
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

type CustomerDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Company   string `json:"company"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func handleAdminCustomersList(w http.ResponseWriter, _ *http.Request, d AdminDeps) {
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	rows, err := d.DB.Query(`select id, email, coalesce(name,''), coalesce(company,''), status, created_at from customers where deleted_at is null order by created_at desc limit 500`)
	if err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var out []CustomerDTO
	for rows.Next() {
		var c CustomerDTO
		var created time.Time
		if err := rows.Scan(&c.ID, &c.Email, &c.Name, &c.Company, &c.Status, &created); err != nil {
			d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		c.CreatedAt = created.Format(time.RFC3339)
		out = append(out, c)
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: out})
}

func handleAdminCustomersCreate(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Company  string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		d.SendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.Email == "" {
		d.SendJSONError(w, "email required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Password) == "" {
		if randPass, err := d.GenerateSessionToken(); err == nil {
			payload.Password = randPass[:16]
		} else {
			payload.Password = "ChangeMe123!"
		}
	}
	hash, err := d.HashPassword(payload.Password)
	if err != nil {
		d.SendJSONError(w, "hash error", http.StatusInternalServerError)
		return
	}
	_, err = d.DB.Exec(`insert into customers(id, email, password_hash, name, company) values(gen_random_uuid(), $1, $2, $3, $4)`,
		payload.Email, hash, payload.Name, payload.Company,
	)
	if err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: map[string]string{"generatedPassword": payload.Password}})
}

func handleAdminCustomersUpdate(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
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
		d.SendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.ID == "" {
		d.SendJSONError(w, "id required", http.StatusBadRequest)
		return
	}
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
		hash, _ := d.HashPassword(*payload.Password)
		idx++
		setParts = append(setParts, fmt.Sprintf("password_hash=$%d", idx))
		args = append(args, hash)
	}
	setParts = append(setParts, "updated_at=now()")
	idx++
	wherePlaceholder := fmt.Sprintf("$%d", idx)
	args = append(args, payload.ID)
	q := "update customers set " + strings.Join(setParts, ", ") + " where id=" + wherePlaceholder
	if _, err := d.DB.Exec(q, args...); err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleAdminCustomersDelete(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		d.SendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if payload.ID == "" {
		d.SendJSONError(w, "id required", http.StatusBadRequest)
		return
	}
	if _, err := d.DB.Exec(`update customers set deleted_at=now() where id=$1 and deleted_at is null`, payload.ID); err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleAdminSubscriptionsCreate(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
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
		d.SendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if p.CustomerID == "" || p.QuotaTotal <= 0 || p.PeriodStart == "" || p.PeriodEnd == "" {
		d.SendJSONError(w, "fields required", http.StatusBadRequest)
		return
	}
	start, err1 := time.Parse("2006-01-02", p.PeriodStart)
	end, err2 := time.Parse("2006-01-02", p.PeriodEnd)
	if err1 != nil || err2 != nil {
		d.SendJSONError(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	if end.Before(start) {
		d.SendJSONError(w, "period_end must be >= period_start", http.StatusBadRequest)
		return
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if end.Before(today) {
		d.SendJSONError(w, "period_end must not be in the past", http.StatusBadRequest)
		return
	}
	auto := true
	if p.AutoRenew != nil {
		auto = *p.AutoRenew
	}
	_, err := d.DB.Exec(`insert into subscriptions(id, customer_id, status, period_start, period_end, quota_total, auto_renew) values(gen_random_uuid(), $1, 'active', $2, $3, $4, $5)`, p.CustomerID, start.Format("2006-01-02"), end.Format("2006-01-02"), p.QuotaTotal, auto)
	if err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success"})
}

func handleAdminCustomersRegeneratePassword(w http.ResponseWriter, r *http.Request, d AdminDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.DB == nil {
		d.SendJSONError(w, "DB not connected", http.StatusServiceUnavailable)
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		d.SendJSONError(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.ID) == "" {
		d.SendJSONError(w, "id required", http.StatusBadRequest)
		return
	}
	token, err := d.GenerateSessionToken()
	if err != nil {
		d.SendJSONError(w, "cannot generate password", http.StatusInternalServerError)
		return
	}
	newPass := token[:16]
	hash, err := d.HashPassword(newPass)
	if err != nil {
		d.SendJSONError(w, "hash error", http.StatusInternalServerError)
		return
	}
	res, err := d.DB.Exec(`update customers set password_hash=$1, updated_at=now() where id=$2 and deleted_at is null`, hash, payload.ID)
	if err != nil {
		d.SendJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		d.SendJSONError(w, "customer not found", http.StatusNotFound)
		return
	}
	d.SendJSONResponse(w, APIResponse{Status: "success", Data: map[string]string{"generatedPassword": newPass}})
}
