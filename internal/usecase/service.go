package usecase

import (
	"context"
	"document-ai/internal/domain"
	"strings"
	"time"
)

type Service struct {
	Publisher  domain.ResultsPublisher
	Dispatcher domain.IntegrationsDispatcher
	LLM        domain.LLMClient
	GenerateID func() string
}

func (s Service) PublishText(text, status, batchID string, seq int, download, userEmail string) {
	resp := domain.ProcessingResponse{
		ID:        s.GenerateID(),
		Text:      text,
		Timestamp: time.Now(),
		Status:    status,
		Download:  download,
		BatchID:   batchID,
		Seq:       seq,
		UserEmail: userEmail,
	}

	if s.Publisher != nil {
		s.Publisher.Publish(resp)
	}

	if status == "completed" && userEmail != "" && s.Dispatcher != nil {
		s.Dispatcher.Dispatch(userEmail, text)
	}
}

// ProcessDocumentAsync запускает LLM и публикует результат
func (s Service) ProcessDocumentAsync(message string, fileName string, contentType string, batchID string, seq int, fileBytes []byte, userEmail string) error {
	if s.LLM == nil {
		return nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		text, err := s.LLM.Process(ctx, message, fileName, contentType, fileBytes)
		if err != nil {
			s.PublishText("Ошибка запроса к модели: "+err.Error(), "error", batchID, seq, "", userEmail)
			return
		}
		if strings.TrimSpace(text) == "" {
			text = "Модель не вернула содержимое ответа"
		}
		s.PublishText(text, "completed", batchID, seq, "", userEmail)
	}()
	return nil
}
