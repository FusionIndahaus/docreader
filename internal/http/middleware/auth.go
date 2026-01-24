package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

type Auth struct {
	GetUserEmail  func(string) string
	AdminSessions map[string]string
	ServiceAPIKey string
}

func (a Auth) RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if email, ok := a.authorizeServiceRequest(r); ok && email != "" {
			next(w, WithUserEmail(r, email))
			return
		}
		c, err := r.Cookie("user_session")
		if err != nil || c.Value == "" || a.GetUserEmail(c.Value) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a Auth) RequireUserOrAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("admin_session"); err == nil && c.Value != "" && a.AdminSessions[c.Value] != "" {
			next(w, r)
			return
		}
		if c, err := r.Cookie("user_session"); err == nil && c.Value != "" && a.GetUserEmail(c.Value) != "" {
			next(w, r)
			return
		}
		if email, ok := a.authorizeServiceRequest(r); ok {
			if email != "" {
				next(w, WithUserEmail(r, email))
				return
			}
			next(w, r)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

func (a Auth) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("admin_session")
		if err != nil || c.Value == "" || a.AdminSessions[c.Value] == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type ctxKey string

const ctxUserEmail ctxKey = "docreaderUserEmail"

func WithUserEmail(r *http.Request, email string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserEmail, strings.ToLower(strings.TrimSpace(email)))
	return r.WithContext(ctx)
}

func UserEmailFromContext(r *http.Request) string {
	if v := r.Context().Value(ctxUserEmail); v != nil {
		if s, ok := v.(string); ok {
			return strings.ToLower(strings.TrimSpace(s))
		}
	}
	return ""
}

func (a Auth) authorizeServiceRequest(r *http.Request) (string, bool) {
	if strings.TrimSpace(a.ServiceAPIKey) == "" {
		return "", false
	}
	apiKey := strings.TrimSpace(r.Header.Get("X-Docreader-Api-Key"))
	if apiKey == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			apiKey = strings.TrimSpace(auth[7:])
		}
	}
	if apiKey == "" {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(apiKey), []byte(a.ServiceAPIKey)) != 1 {
		return "", false
	}
	email := strings.TrimSpace(r.Header.Get("X-Docreader-User"))
	if email == "" {
		email = strings.TrimSpace(r.Header.Get("X-User-Email"))
	}
	return strings.ToLower(email), true
}
