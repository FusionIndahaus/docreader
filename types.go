package main

import "time"

type DocumentRequest struct {
	Message  string `json:"message"`
	FileName string `json:"fileName"`
}

type ProcessingResponse struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
