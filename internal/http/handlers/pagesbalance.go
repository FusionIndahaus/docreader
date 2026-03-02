package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	authmw "document-ai/internal/http/middleware"
	"document-ai/internal/pagesbalance"
)

// PagesBalanceDeps holds dependencies for the pages-balance handlers.
type PagesBalanceDeps struct {
	DB               *sql.DB
	RequireUser      func(http.HandlerFunc) http.HandlerFunc
	GetUserEmail     func(string) string
	SendJSONResponse func(http.ResponseWriter, interface{})
	SendJSONError    func(http.ResponseWriter, string, int)
}

// RegisterPagesBalanceRoutes registers internal and user-facing endpoints for pages balance.
//
//	POST /internal/pages-balance/init   — create trial record (tarif_id=1, amount=10)
//	POST /internal/pages-balance/update — upsert record when subscription changes
//	GET  /user/pages-balance            — get current balance for authenticated user
func RegisterPagesBalanceRoutes(mux *http.ServeMux, d PagesBalanceDeps) {
	// Internal: init balance for new user (called by billing service)
	mux.HandleFunc("/internal/pages-balance/init", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			d.SendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
			d.SendJSONError(w, "user_id is required", http.StatusBadRequest)
			return
		}

		if err := pagesbalance.InitUserBalance(d.DB, body.UserID); err != nil {
			d.SendJSONError(w, "failed to init pages balance: "+err.Error(), http.StatusInternalServerError)
			return
		}

		d.SendJSONResponse(w, map[string]interface{}{
			"success":  true,
			"user_id":  body.UserID,
			"tarif_id": 1,
			"amount":   pagesbalance.AmountByTarif[1],
		})
	})

	// Internal: update balance when subscription changes (called by billing service)
	mux.HandleFunc("/internal/pages-balance/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			d.SendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			UserID  string `json:"user_id"`
			TarifID int    `json:"tarif_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || body.TarifID == 0 {
			d.SendJSONError(w, "user_id and tarif_id are required", http.StatusBadRequest)
			return
		}

		if err := pagesbalance.UpdateUserBalance(d.DB, body.UserID, body.TarifID); err != nil {
			d.SendJSONError(w, "failed to update pages balance: "+err.Error(), http.StatusInternalServerError)
			return
		}

		d.SendJSONResponse(w, map[string]interface{}{
			"success":  true,
			"user_id":  body.UserID,
			"tarif_id": body.TarifID,
			"amount":   pagesbalance.AmountByTarif[body.TarifID],
		})
	})

	// User-facing: get current balance for the authenticated user
	mux.HandleFunc("/user/pages-balance", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			d.SendJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := authmw.UserIDFromContext(r)
		if userID == "" {
			email := getRequestUserEmail(r, d.GetUserEmail)
			if email == "" {
				d.SendJSONError(w, "could not identify user", http.StatusUnauthorized)
				return
			}
			userID = email
		}

		amount, tarifID, err := pagesbalance.GetBalance(d.DB, userID)
		if err == sql.ErrNoRows {
			d.SendJSONResponse(w, map[string]interface{}{
				"success":  true,
				"user_id":  userID,
				"amount":   0,
				"tarif_id": 0,
			})
			return
		}
		if err != nil {
			d.SendJSONError(w, "failed to get pages balance: "+err.Error(), http.StatusInternalServerError)
			return
		}

		d.SendJSONResponse(w, map[string]interface{}{
			"success":  true,
			"user_id":  userID,
			"amount":   amount,
			"tarif_id": tarifID,
		})
	}))
}
