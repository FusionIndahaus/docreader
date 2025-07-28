package onec

import (
	"testing"
	"time"
)

// Тестируем создание нового клиента 1С
func TestNewOneCClient(t *testing.T) {
	config := OneCConfig{
		BaseURL:  "http://localhost:8080",
		Username: "testuser",
		Password: "testpass",
		Timeout:  30,
	}

	client := NewOneCClient(config)

	if client == nil {
		t.Error("NewOneCClient не должен возвращать nil")
	}

	if client.BaseURL != config.BaseURL {
		t.Errorf("BaseURL не совпадает: получен %s, ожидался %s", client.BaseURL, config.BaseURL)
	}

	if client.Username != config.Username {
		t.Errorf("Username не совпадает: получен %s, ожидался %s", client.Username, config.Username)
	}

	if client.Password != config.Password {
		t.Errorf("Password не совпадает: получен %s, ожидался %s", client.Password, config.Password)
	}
}

// Тестируем создание клиента с таймаутом по умолчанию
func TestNewOneCClientDefaultTimeout(t *testing.T) {
	config := OneCConfig{
		BaseURL:  "http://localhost:8080",
		Username: "testuser",
		Password: "testpass",
		Timeout:  0, // должен использоваться дефолт
	}

	client := NewOneCClient(config)

	if client.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("Timeout по умолчанию должен быть 30 секунд, получен %v", client.HTTPClient.Timeout)
	}
}

// Тестируем создание клиента с кастомным таймаутом
func TestNewOneCClientCustomTimeout(t *testing.T) {
	config := OneCConfig{
		BaseURL:  "http://localhost:8080",
		Username: "testuser",
		Password: "testpass",
		Timeout:  60,
	}

	client := NewOneCClient(config)

	if client.HTTPClient.Timeout != 60*time.Second {
		t.Errorf("Кастомный timeout должен быть 60 секунд, получен %v", client.HTTPClient.Timeout)
	}
}

// Тестируем структуру DocumentData
func TestDocumentDataCreation(t *testing.T) {
	metadata := DocumentMetadata{
		OriginalName: "test.pdf",
		FileSize:     1024,
		ProcessedBy:  "AI",
		Confidence:   0.95,
	}

	docData := DocumentData{
		ID:           "test-123",
		DocumentType: "invoice",
		CreatedAt:    time.Now(),
		Fields: map[string]interface{}{
			"amount": 1000.50,
			"vendor": "ООО Тест",
		},
		Metadata: metadata,
	}

	if docData.ID != "test-123" {
		t.Errorf("ID не совпадает: получен %s, ожидался test-123", docData.ID)
	}

	if docData.DocumentType != "invoice" {
		t.Errorf("DocumentType не совпадает: получен %s, ожидался invoice", docData.DocumentType)
	}

	if docData.Metadata.OriginalName != "test.pdf" {
		t.Errorf("OriginalName не совпадает: получен %s, ожидался test.pdf", docData.Metadata.OriginalName)
	}

	if docData.Metadata.FileSize != 1024 {
		t.Errorf("FileSize не совпадает: получен %d, ожидался 1024", docData.Metadata.FileSize)
	}

	if docData.Metadata.Confidence != 0.95 {
		t.Errorf("Confidence не совпадает: получен %f, ожидался 0.95", docData.Metadata.Confidence)
	}
}

// Тестируем структуру OneCResponse
func TestOneCResponseCreation(t *testing.T) {
	response := OneCResponse{
		Success:   true,
		Message:   "Документ создан успешно",
		Reference: "00000001-0001-0001-0001-000000000001",
	}

	if !response.Success {
		t.Error("Success должен быть true")
	}

	if response.Message != "Документ создан успешно" {
		t.Errorf("Message не совпадает: получен %s", response.Message)
	}

	if response.Reference == "" {
		t.Error("Reference не должен быть пустым")
	}
}

// Тестируем структуру OneCResponse с ошибкой
func TestOneCResponseError(t *testing.T) {
	response := OneCResponse{
		Success: false,
		Message: "Ошибка обработки",
		Error:   "Timeout connecting to 1C",
	}

	if response.Success {
		t.Error("Success должен быть false для ошибки")
	}

	if response.Error == "" {
		t.Error("Error не должен быть пустым при ошибке")
	}
}

// Тестируем валидацию метаданных документа
func TestDocumentMetadataValidation(t *testing.T) {
	metadata := DocumentMetadata{
		OriginalName: "",
		FileSize:     -1,
		ProcessedBy:  "",
		Confidence:   -0.5,
	}

	// Проверяем что структура может содержать некорректные данные
	// (валидация должна быть на уровне бизнес-логики)
	if metadata.FileSize >= 0 {
		t.Error("FileSize может быть отрицательным (валидация на уровне сервиса)")
	}

	if metadata.Confidence >= 0 {
		t.Error("Confidence может быть отрицательным (валидация на уровне сервиса)")
	}
}

// Benchmark для создания клиента
func BenchmarkNewOneCClient(b *testing.B) {
	config := OneCConfig{
		BaseURL:  "http://localhost:8080",
		Username: "testuser",
		Password: "testpass",
		Timeout:  30,
	}

	for i := 0; i < b.N; i++ {
		NewOneCClient(config)
	}
}

// Benchmark для создания DocumentData
func BenchmarkDocumentDataCreation(b *testing.B) {
	metadata := DocumentMetadata{
		OriginalName: "test.pdf",
		FileSize:     1024,
		ProcessedBy:  "AI",
		Confidence:   0.95,
	}

	for i := 0; i < b.N; i++ {
		_ = DocumentData{
			ID:           "test-123",
			DocumentType: "invoice",
			CreatedAt:    time.Now(),
			Fields: map[string]interface{}{
				"amount": 1000.50,
				"vendor": "ООО Тест",
			},
			Metadata: metadata,
		}
	}
}
