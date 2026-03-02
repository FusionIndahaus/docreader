package domain

import (
	"context"
	"time"
)

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

type ResultsPublisher interface {
	Publish(resp ProcessingResponse)
}

type IntegrationsDispatcher interface {
	Dispatch(userEmail string, text string)
}

type CustomerRepository interface {
	GetIDByEmail(email string) (string, error)
}

type LLMClient interface {
	// Process принимает описание задания и файл, возвращает извлечённый текст
	Process(ctx context.Context, message string, fileName string, contentType string, fileBytes []byte) (string, error)
}
