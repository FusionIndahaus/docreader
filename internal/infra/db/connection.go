package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// Init открывает подключение к Postgres с ретраями.
func Init(dsn string) (*sql.DB, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		d, err := sql.Open("postgres", dsn)
		if err == nil {
			if err = d.Ping(); err == nil {
				log.Println("DB connected")
				return d, nil
			}
			d.Close()
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func Close(d *sql.DB) {
	if d != nil {
		_ = d.Close()
	}
}
