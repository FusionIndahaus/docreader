package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

func TestRegisterResultsRoutes_OK(t *testing.T) {
	mux := http.NewServeMux()
	// test data
	userEmail := "test@example.com"
	token := "tok"
	results := []map[string]interface{}{
		{"id": "1", "text": "ok", "timestamp": time.Now(), "status": "completed", "user_email": userEmail},
		{"id": "2", "text": "skip other", "timestamp": time.Now(), "status": "completed", "user_email": "other@example.com"},
	}
	RegisterResultsRoutes(mux, ResultsDeps{
		RequireUser:      func(h http.HandlerFunc) http.HandlerFunc { return h },
		SendJSONResponse: func(w http.ResponseWriter, v interface{}) { _ = json.NewEncoder(w).Encode(v) },
		SendJSONError: func(w http.ResponseWriter, msg string, code int) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": msg})
		},
		GetUserEmail: func(tok string) string {
			if tok == token {
				return userEmail
			}
			return ""
		},
		GetUserResults: func(email string) interface{} {
			var out []map[string]interface{}
			for _, r := range results {
				if strings.EqualFold(email, r["user_email"].(string)) {
					out = append(out, r)
				}
			}
			return out
		},
	})
	req, _ := http.NewRequest("GET", "/results", nil)
	req.AddCookie(&http.Cookie{Name: "user_session", Value: token})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload testResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Status != "success" {
		t.Fatalf("expected status=success, got %s", payload.Status)
	}
	// Ensure only own results returned
	b, _ := json.Marshal(payload.Data)
	var arr []map[string]interface{}
	_ = json.Unmarshal(b, &arr)
	if len(arr) != 1 {
		t.Fatalf("expected 1 result, got %d", len(arr))
	}
}

func TestRegisterResultsRoutes_Unauthorized(t *testing.T) {
	mux := http.NewServeMux()
	RegisterResultsRoutes(mux, ResultsDeps{
		RequireUser:      func(h http.HandlerFunc) http.HandlerFunc { return h },
		SendJSONResponse: func(w http.ResponseWriter, v interface{}) { _ = json.NewEncoder(w).Encode(v) },
		SendJSONError: func(w http.ResponseWriter, msg string, code int) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": msg})
		},
		GetUserEmail:   func(tok string) string { return "" },
		GetUserResults: func(email string) interface{} { return nil },
	})
	req, _ := http.NewRequest("GET", "/results", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
