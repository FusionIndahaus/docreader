package main

import "time"

type ProcessingResponse struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Download  string    `json:"download,omitempty"`
	BatchID   string    `json:"batchId,omitempty"`
	Seq       int       `json:"seq,omitempty"`
	UserEmail string    `json:"user_email,omitempty"`
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
