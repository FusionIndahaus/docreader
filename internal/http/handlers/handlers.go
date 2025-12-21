package handlers

import (
	"context"
	"database/sql"
	amocrmpkg "document-ai/internal/integrations/amocrm"
	"net/http"
	"strings"
	"time"
)

type Deps struct {
	DB                   *sql.DB
	RequireUser          func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse     func(http.ResponseWriter, interface{})
	SendJSONError        func(http.ResponseWriter, string, int)
	GetUserEmail         func(string) string
	GetCustomerIDByEmail func(string) (string, error)
	GenerateSessionToken func() (string, error)
	SafeCompareStrings   func(string, string) bool
	AmoRedirectURI       string
}

func RegisterAmoUserRoutes(mux *http.ServeMux, d Deps) {
	mux.HandleFunc("/user/amocrm/status", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		handleUserAmoStatus(w, r, d)
	}))
	mux.HandleFunc("/user/amocrm/connect", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		handleUserAmoConnect(w, r, d)
	}))
	mux.HandleFunc("/user/amocrm/oauth/callback", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		handleUserAmoOAuthCallback(w, r, d)
	}))
}

func handleUserAmoStatus(w http.ResponseWriter, r *http.Request, d Deps) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || d.GetUserEmail(c.Value) == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := d.GetUserEmail(c.Value)
	if !amocrmpkg.IsConfigured() {
		d.SendJSONResponse(w, map[string]interface{}{"status": "success", "data": map[string]interface{}{"configured": false, "connected": false}})
		return
	}
	customerID, err := d.GetCustomerIDByEmail(email)
	if err != nil {
		d.SendJSONResponse(w, map[string]interface{}{"status": "success", "data": map[string]interface{}{"configured": true, "connected": false, "error": "no_customer"}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tok, err := amocrmpkg.LoadCustomerTokens(ctx, d.DB, customerID)
	if err != nil || tok.AccessToken == "" {
		d.SendJSONResponse(w, map[string]interface{}{"status": "success", "data": map[string]interface{}{"configured": true, "connected": false}})
		return
	}
	d.SendJSONResponse(w, map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"configured":  true,
			"connected":   true,
			"expires_at":  tok.ExpiresAt.Format(time.RFC3339),
			"account_url": tok.AccountDomain,
		},
	})
}

func handleUserAmoConnect(w http.ResponseWriter, r *http.Request, d Deps) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || d.GetUserEmail(c.Value) == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, err := d.GenerateSessionToken()
	if err != nil {
		d.SendJSONError(w, "state error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "amo_state",
		Value:    state,
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(10 * time.Minute),
	})
	email := d.GetUserEmail(c.Value)
	customerID, err := d.GetCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		d.SendJSONError(w, "user not found", http.StatusBadRequest)
		return
	}
	url, err := amocrmpkg.GetAuthorizeURLForCustomer(r.Context(), d.DB, customerID, state, d.AmoRedirectURI)
	if err != nil {
		d.SendJSONError(w, "prepare auth url error: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func handleUserAmoOAuthCallback(w http.ResponseWriter, r *http.Request, d Deps) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || d.GetUserEmail(c.Value) == "" {
		d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := d.GetUserEmail(c.Value)
	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))
	state := strings.TrimSpace(q.Get("state"))
	if code == "" || state == "" {
		d.SendJSONError(w, "missing code/state", http.StatusBadRequest)
		return
	}
	sc, err := r.Cookie("amo_state")
	if err != nil || sc.Value == "" || !d.SafeCompareStrings(sc.Value, state) {
		d.SendJSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	sc.Expires = time.Unix(0, 0)
	sc.MaxAge = -1
	http.SetCookie(w, sc)

	customerID, err := d.GetCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		d.SendJSONError(w, "customer not found", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := amocrmpkg.ExchangeAuthCodeForCustomer(ctx, d.DB, customerID, code, d.AmoRedirectURI); err != nil {
		d.SendJSONError(w, "oauth exchange error: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/user/dashboard", http.StatusFound)
		return
	}
	d.SendJSONResponse(w, map[string]interface{}{"status": "success", "message": "amoCRM connected"})
}
