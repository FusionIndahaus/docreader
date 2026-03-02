package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"
)

func RegisterHealth(mux *http.ServeMux, sendJSON func(http.ResponseWriter, interface{})) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":     "healthy",
			"message":    "Document AI работает нормально",
			"timestamp":  time.Now().Format(time.RFC3339),
			"version":    "2.0.0",
			"db_enabled": strings.TrimSpace(os.Getenv("DATABASE_URL")) != "",
		}
		sendJSON(w, map[string]interface{}{
			"status": "success",
			"data":   health,
		})
	})
}
