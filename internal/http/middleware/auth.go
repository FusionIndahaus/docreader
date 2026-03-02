package middleware

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Auth struct {
	GetUserEmail  func(string) string
	AdminSessions map[string]string
	ServiceAPIKey string
}

func (a Auth) RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверяем JWT токен в Authorization header
		if userID, email, ok := a.authorizeJWT(r); ok {
			req := WithUserID(r, userID)
			req = WithUserEmail(req, email)
			next(w, req)
			return
		}
		// Fallback: service-to-service auth (старый метод)
		if email, ok := a.authorizeServiceRequest(r); ok && email != "" {
			next(w, WithUserEmail(r, email))
			return
		}
		// Fallback: cookie auth
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
		// JWT auth: API key + Bearer token (used by backend-gateway on behalf of users)
		if userID, email, ok := a.authorizeJWT(r); ok {
			req := WithUserID(r, userID)
			req = WithUserEmail(req, email)
			next(w, req)
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
const ctxUserID ctxKey = "docreaderUserID"

func WithUserEmail(r *http.Request, email string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserEmail, strings.ToLower(strings.TrimSpace(email)))
	return r.WithContext(ctx)
}

func WithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserID, userID)
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

func UserIDFromContext(r *http.Request) string {
	if v := r.Context().Value(ctxUserID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// authorizeJWT проверяет JWT токен и возвращает userID и email
func (a Auth) authorizeJWT(r *http.Request) (string, string, bool) {
	// Проверяем API ключ
	apiKey := strings.TrimSpace(r.Header.Get("X-Docreader-Api-Key"))
	if apiKey == "" {
		log.Println("❌ [JWT Auth] X-Docreader-Api-Key header is missing")
		return "", "", false
	}
	if strings.TrimSpace(a.ServiceAPIKey) == "" {
		log.Println("❌ [JWT Auth] ServiceAPIKey not configured in server")
		return "", "", false
	}
	if subtle.ConstantTimeCompare([]byte(apiKey), []byte(a.ServiceAPIKey)) != 1 {
		log.Printf("❌ [JWT Auth] API key mismatch. Got: %s..., Expected: %s...\n", 
			apiKey[:min(8, len(apiKey))], 
			a.ServiceAPIKey[:min(8, len(a.ServiceAPIKey))])
		return "", "", false
	}
	log.Println("✅ [JWT Auth] API key validated")

	// Извлекаем JWT токен из Authorization header
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		log.Println("❌ [JWT Auth] Authorization header is missing")
		return "", "", false
	}
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		log.Println("❌ [JWT Auth] Authorization header does not start with 'Bearer '")
		return "", "", false
	}
	tokenString := strings.TrimSpace(auth[7:])
	if tokenString == "" {
		log.Println("❌ [JWT Auth] Token string is empty after 'Bearer '")
		return "", "", false
	}
	log.Printf("✅ [JWT Auth] Token received: %s...\n", tokenString[:min(20, len(tokenString))])

	// Парсим и проверяем JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("❌ [JWT Auth] JWT_SECRET environment variable is not set")
		return "", "", false
	}
	log.Printf("✅ [JWT Auth] JWT_SECRET is set: %s...\n", jwtSecret[:min(10, len(jwtSecret))])

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("❌ [JWT Auth] Invalid signing method: %v\n", token.Method)
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		log.Printf("❌ [JWT Auth] JWT parse error: %v\n", err)
		return "", "", false
	}
	if !token.Valid {
		log.Println("❌ [JWT Auth] Token is not valid")
		return "", "", false
	}
	log.Println("✅ [JWT Auth] Token parsed and validated successfully")

	// Извлекаем claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		log.Printf("📋 [JWT Auth] Claims: %+v\n", claims)
		
		userID, _ := claims["userId"].(string)
		email, _ := claims["email"].(string)
		
		log.Printf("🔍 [JWT Auth] Extracted - userID: %q, email: %q\n", userID, email)
		
		if userID != "" && email != "" {
			log.Printf("✅ [JWT Auth] Successfully authenticated user: %s (%s)\n", email, userID)
			return userID, email, true
		}
		log.Println("❌ [JWT Auth] userId or email is empty in claims")
	} else {
		log.Println("❌ [JWT Auth] Failed to extract claims as MapClaims")
	}

	return "", "", false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
