package main

import (
	"database/sql"
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
