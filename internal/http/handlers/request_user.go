package handlers

import (
	"net/http"
	"strings"

	authmw "document-ai/internal/http/middleware"
)

func getRequestUserEmail(r *http.Request, getUserEmail func(string) string) string {
	if email := authmw.UserEmailFromContext(r); email != "" {
		return strings.ToLower(strings.TrimSpace(email))
	}
	if c, err := r.Cookie("user_session"); err == nil && c.Value != "" {
		if email := getUserEmail(c.Value); email != "" {
			return strings.ToLower(strings.TrimSpace(email))
		}
	}
	return ""
}
