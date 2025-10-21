package main

import (
	"database/sql"
	"log"
	"time"
)

func initDatabase() error {
	var lastErr error
	for i := 0; i < 30; i++ { // до ~60 секунд ожидания
		d, err := sql.Open("postgres", dbDSN)
		if err == nil {
			if err = d.Ping(); err == nil {
				db = d
				log.Println("DB connected")
				return nil
			}
			d.Close()
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return lastErr
}

func closeDatabase() {
	if db != nil {
		db.Close()
	}
}
