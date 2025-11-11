package main

import (
	"database/sql"
	"sync"
)

var (
	responses      []ProcessingResponse
	responsesMutex sync.RWMutex
	subscribers    map[chan ProcessingResponse]struct{}
	subscribersMux sync.RWMutex

	userSessions    = map[string]string{}
	userSessionsMux sync.RWMutex

	db *sql.DB
)
