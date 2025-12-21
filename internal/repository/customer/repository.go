package customer

import (
	"database/sql"
	"strings"
)

// GetIDByEmail возвращает идентификатор клиента по email.
func GetIDByEmail(db *sql.DB, email string) (string, error) {
	if db == nil {
		return "", sql.ErrConnDone
	}
	email = strings.TrimSpace(email)
	var id string
	row := db.QueryRow(`select id from customers where deleted_at is null and lower(email)=lower($1)`, email)
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}
