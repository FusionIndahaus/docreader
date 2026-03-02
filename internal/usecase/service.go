package usecase

import (
	"context"
	"database/sql"
	"document-ai/internal/domain"
	"document-ai/internal/pagesbalance"
	"log"
	"strings"
	"time"
)

type Service struct {
	Publisher         domain.ResultsPublisher
	Dispatcher        domain.IntegrationsDispatcher
	LLM               domain.LLMClient
	GenerateID        func() string
	DB                *sql.DB
	BillingServiceURL string
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

// ProcessDocumentAsync запускает LLM асинхронно и публикует результат.
// После получения ответа от нейросети:
//   - при успехе  — списывает страницы и записывает status='success' в pages_details.
//   - при ошибке  — записывает status='error' в pages_details.
//
// userID    — числовой ID из JWT (совпадает с pages_balance.user_id).
// batchID   — ID группы файлов одного запроса (из FormData).
// pagesCount — количество страниц файла (для списания).
func (s Service) ProcessDocumentAsync(
	message, fileName, contentType, batchID string,
	seq int,
	fileBytes []byte,
	userEmail, userID string,
	pagesCount int,
) error {
	if s.LLM == nil {
		println("⚠️ WARNING: LLM client is nil, skipping processing")
		return nil
	}
	println("🔄 Starting async document processing for:", fileName, "user:", userEmail)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		println("📞 Calling LLM.Process for:", fileName)
		text, err := s.LLM.Process(ctx, message, fileName, contentType, fileBytes)

		if err != nil {
			println("❌ LLM.Process error:", err.Error())
			// Mark batch as failed
			if s.DB != nil && batchID != "" {
				if dbErr := pagesbalance.UpdatePagesDetailStatus(s.DB, batchID, "error"); dbErr != nil {
					log.Printf("⚠️ [usecase] failed to update pages_detail status to error: %v", dbErr)
				}
			}
			s.PublishText("Ошибка запроса к модели: "+err.Error(), "error", batchID, seq, "", userEmail)
			return
		}

		println("✅ LLM.Process completed for:", fileName, "text length:", len(text))
		if strings.TrimSpace(text) == "" {
			text = "Модель не вернула содержимое ответа"
		}

		// ── Списать страницы ТОЛЬКО после успешного ответа нейросети ──────────
		if pagesCount > 0 && userID != "" && s.DB != nil {
			remaining, deductErr := pagesbalance.DeductPages(s.DB, userID, pagesCount)
			if deductErr != nil {
				log.Printf("⚠️ [usecase] failed to deduct %d pages for user %s: %v", pagesCount, userID, deductErr)
			} else {
				log.Printf("📊 [usecase] deducted %d pages for user %s, remaining: %d", pagesCount, userID, remaining)
				if remaining == 0 && s.BillingServiceURL != "" {
					go pagesbalance.NotifyBillingZeroBalance(s.BillingServiceURL, userID)
				}
			}
		}

		// ── Обновить статус детализации ──────────────────────────────────────
		if s.DB != nil && batchID != "" {
			if dbErr := pagesbalance.UpdatePagesDetailStatus(s.DB, batchID, "success"); dbErr != nil {
				log.Printf("⚠️ [usecase] failed to update pages_detail status to success: %v", dbErr)
			}
		}

		println("📢 Publishing result for user:", userEmail)
		s.PublishText(text, "completed", batchID, seq, "", userEmail)
		println("✅ Result published successfully")
	}()

	return nil
}
