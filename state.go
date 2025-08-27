package main

import "sync"

var (
	responses      []ProcessingResponse
	responsesMutex sync.RWMutex
	subscribers    map[chan ProcessingResponse]struct{}
	subscribersMux sync.RWMutex
)
