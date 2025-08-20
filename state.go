package main

import "sync"

var (
	responses      []ProcessingResponse
	responsesMutex sync.RWMutex
)
