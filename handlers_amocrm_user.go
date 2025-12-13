package main

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Статус пользовательского подключения amoCRM
func handleUserAmoStatus(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	if !isAmoConfigured() {
		sendJSONResponse(w, APIResponse{
			Status: "success",
			Data: map[string]interface{}{
				"configured": false,
				"connected":  false,
			},
		})
		return
	}
	customerID, err := getCustomerIDByEmail(email)
	if err != nil {
		sendJSONResponse(w, APIResponse{
			Status: "success",
			Data: map[string]interface{}{
				"configured": true,
				"connected":  false,
				"error":      "no_customer",
			},
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tok, err := loadCustomerAmoTokens(ctx, customerID)
	if err != nil || tok.AccessToken == "" {
		sendJSONResponse(w, APIResponse{
			Status: "success",
			Data: map[string]interface{}{
				"configured": true,
				"connected":  false,
			},
		})
		return
	}
	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data: map[string]interface{}{
			"configured":  true,
			"connected":   true,
			"expires_at":  tok.ExpiresAt.Format(time.RFC3339),
			"account_url": tok.AccountDomain,
		},
	})
}

// Редирект на amoCRM OAuth (для пользователя)
func handleUserAmoConnect(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, err := generateSessionToken()
	if err != nil {
		sendJSONError(w, "state error", http.StatusInternalServerError)
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
	email := getUserEmail(c.Value)
	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "user not found", http.StatusBadRequest)
		return
	}
	url, err := getAmoAuthorizeURLForCustomer(r.Context(), customerID, state)
	if err != nil {
		sendJSONError(w, "prepare auth url error: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback OAuth (для пользователя)
func handleUserAmoOAuthCallback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" || getUserEmail(c.Value) == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	email := getUserEmail(c.Value)
	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))
	state := strings.TrimSpace(q.Get("state"))
	if code == "" || state == "" {
		sendJSONError(w, "missing code/state", http.StatusBadRequest)
		return
	}
	sc, err := r.Cookie("amo_state")
	if err != nil || sc.Value == "" || !safeCompareStrings(sc.Value, state) {
		sendJSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	sc.Expires = time.Unix(0, 0)
	sc.MaxAge = -1
	http.SetCookie(w, sc)

	customerID, err := getCustomerIDByEmail(email)
	if err != nil || customerID == "" {
		sendJSONError(w, "customer not found", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := exchangeAmoAuthCodeForCustomer(ctx, customerID, code); err != nil {
		sendJSONError(w, "oauth exchange error: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/user/dashboard", http.StatusFound)
		return
	}
	sendJSONResponse(w, APIResponse{Status: "success", Message: "amoCRM connected"})
}
