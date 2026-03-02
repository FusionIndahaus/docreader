package pagesbalance

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// AmountByTarif maps tarif_id → pages limit for the autoaccounter product.
// Must stay in sync with pagesBalance.service.ts in the billing service.
var AmountByTarif = map[int]int{
	1: 10,   // Trial
	2: 1500, // Start
	3: 1800, // Business
	4: 2500, // Ultimate
}

// InitUserBalance inserts a trial balance record (tarif_id=1, amount=10) for a new user.
// Uses ON CONFLICT DO NOTHING so repeated calls are safe.
func InitUserBalance(db *sql.DB, userID string) error {
	_, err := db.Exec(`
		INSERT INTO pages_balance (user_id, tarif_id, amount)
		VALUES ($1, 1, 10)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	return err
}

// UpdateUserBalance upserts pages_balance for a user when their subscription changes.
// Returns an error if tarifID is not in AmountByTarif.
func UpdateUserBalance(db *sql.DB, userID string, tarifID int) error {
	amount, ok := AmountByTarif[tarifID]
	if !ok {
		return fmt.Errorf("unknown tarif_id %d: no pages amount configured", tarifID)
	}

	_, err := db.Exec(`
		INSERT INTO pages_balance (user_id, tarif_id, amount)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		    SET tarif_id   = EXCLUDED.tarif_id,
		        amount     = EXCLUDED.amount
	`, userID, tarifID, amount)
	return err
}

// DeductPages reduces the user's page balance by count (minimum 0).
// Returns the remaining balance after deduction.
// Returns sql.ErrNoRows if no balance record exists for the user.
func DeductPages(db *sql.DB, userID string, count int) (remaining int, err error) {
	err = db.QueryRow(`
		UPDATE pages_balance
		SET amount = GREATEST(0, amount - $2)
		WHERE user_id = $1
		RETURNING amount
	`, userID, count).Scan(&remaining)
	return
}

// GetBalance returns the current page balance and tarif for a user.
// Returns sql.ErrNoRows if no balance record exists for the user.
func GetBalance(db *sql.DB, userID string) (amount int, tarifID int, err error) {
	err = db.QueryRow(`
		SELECT amount, tarif_id FROM pages_balance WHERE user_id = $1
	`, userID).Scan(&amount, &tarifID)
	return
}

// UpsertPagesDetail creates or updates a pages_details record for a batch.
//   - If batch_id is empty: always inserts a new row.
//   - If batch_id is set: upserts — appends docType (if not already present),
//     adds pagesCount to the running total, and increments files_count.
//
// Returns the id of the affected row.
func UpsertPagesDetail(db *sql.DB, userID, batchID, docType string, pagesCount int) (id int64, err error) {
	if batchID == "" {
		// No batch grouping — plain insert
		err = db.QueryRow(`
			INSERT INTO pages_details (user_id, doc_types, pages_count, files_count, status, created_at)
			VALUES ($1, $2, $3, 1, 'pending', NOW())
			RETURNING id
		`, userID, docType, pagesCount).Scan(&id)
		return
	}

	// Upsert: accumulate doc_types (skip duplicate), sum pages_count, increment files_count.
	// POSITION($3 IN doc_types) > 0 is safe for our short type names (pdf, docx, jpg, png).
	err = db.QueryRow(`
		INSERT INTO pages_details (user_id, batch_id, doc_types, pages_count, files_count, status, created_at)
		VALUES ($1, $2, $3, $4, 1, 'pending', NOW())
		ON CONFLICT (batch_id) DO UPDATE
		    SET doc_types   = CASE
		                        WHEN POSITION($3 IN pages_details.doc_types) > 0
		                        THEN pages_details.doc_types
		                        ELSE pages_details.doc_types || ',' || $3
		                      END,
		        pages_count  = pages_details.pages_count + $4,
		        files_count  = pages_details.files_count + 1
		RETURNING id
	`, userID, batchID, docType, pagesCount).Scan(&id)
	return
}

// UpdatePagesDetailStatus updates the status of a pages_details record by batchID.
// Priority rule (handled in SQL):
//   - 'error' always overwrites any status.
//   - 'success' only updates from 'pending' (won't downgrade from 'error').
//   - 'insufficient_pages' always overwrites 'pending'.
//
// When batchID is empty the call is a no-op.
func UpdatePagesDetailStatus(db *sql.DB, batchID, status string) error {
	if batchID == "" {
		return nil
	}
	_, err := db.Exec(`
		UPDATE pages_details
		SET status = CASE
		               WHEN $2 = 'error'               THEN 'error'
		               WHEN $2 = 'insufficient_pages'  THEN 'insufficient_pages'
		               WHEN $2 = 'success' AND status = 'pending' THEN 'success'
		               ELSE status
		             END
		WHERE batch_id  = $1
		  AND deleted_at IS NULL
	`, batchID, status)
	return err
}

// NotifyBillingZeroBalance calls the billing service to set the autoaccounter
// subscription to inactive when a user's page balance reaches 0.
// Designed to run inside a goroutine — errors are only logged, never returned.
func NotifyBillingZeroBalance(billingURL, userID string) {
	if billingURL == "" || userID == "" {
		return
	}
	body, _ := json.Marshal(map[string]interface{}{
		"user_id":    userID,
		"product_id": 1,
	})
	url := fmt.Sprintf("%s/api/subscriptions/deactivate-by-pages", billingURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("⚠️ [pagesbalance] billing notification failed for user %s: %v", userID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("⚠️ [pagesbalance] billing notification returned %d for user %s", resp.StatusCode, userID)
		return
	}
	log.Printf("✅ [pagesbalance] billing notified: subscription set inactive for user %s", userID)
}
