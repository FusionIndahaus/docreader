package middleware

import "net/http"

type Auth struct {
	GetUserEmail  func(string) string
	AdminSessions map[string]string
}

func (a Auth) RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
