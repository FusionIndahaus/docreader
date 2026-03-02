package main

import (
	"database/sql"
	domain "document-ai/internal/domain"
	"sync"
)

var (
	responses      []domain.ProcessingResponse
	responsesMutex sync.RWMutex
	subscribers    map[chan domain.ProcessingResponse]struct{}
	subscribersMux sync.RWMutex

	userSessions    = map[string]string{}
	userSessionsMux sync.RWMutex

	// простые админ-сессии
	adminSessions = map[string]string{}

	db *sql.DB
)
