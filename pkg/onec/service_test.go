package onec

import (
	"encoding/json"
	"os"
	"testing"
)

// Создаём тестовый конфигурационный файл
func createTestConfig(t *testing.T) string {
	config := IntegrationConfig{
		OneCConfig: OneCConfig{
			BaseURL:  "http://localhost:8080",
			Username: "testuser",
			Password: "testpass",
			Timeout:  30,
		},
		Enabled:  true,
		AutoSend: true,
		Mapping: map[string]DocumentTypeConfig{
			"invoice": {
				TargetObject: "Document.IncomingInvoice",
				Fields: map[string]string{
					"amount": "Amount",
					"vendor": "Vendor",
				},
			},
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Не удалось сериализовать конфигурацию: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "onec_test_config_*.json")
	if err != nil {
		t.Fatalf("Не удалось создать временный файл: %v", err)
	}

	if _, err := tmpFile.Write(configData); err != nil {
		t.Fatalf("Не удалось записать конфигурацию: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Тестируем создание сервиса с валидной конфигурацией
func TestNewOneCServiceValidConfig(t *testing.T) {
	configPath := createTestConfig(t)
	defer os.Remove(configPath)

	service, err := NewOneCService(configPath)
	if err != nil {
		t.Fatalf("NewOneCService вернул ошибку: %v", err)
	}

	if service == nil {
		t.Error("NewOneCService не должен возвращать nil")
	}

	// Проверяем что сервис создался с правильной конфигурацией
	if service.config == nil {
		t.Error("config не должен быть nil")
	}

	if service.config.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL не совпадает: получен %s", service.config.BaseURL)
	}
}

// Тестируем создание сервиса с несуществующим файлом конфигурации
func TestNewOneCServiceInvalidConfig(t *testing.T) {
	service, err := NewOneCService("nonexistent_config.json")
	// loadConfig не возвращает ошибку для несуществующего файла,
	// а создаёт конфигурацию по умолчанию
	if err != nil {
		t.Errorf("NewOneCService не должен возвращать ошибку для несуществующего файла: %v", err)
	}

	if service == nil {
		t.Error("NewOneCService не должен возвращать nil")
	}

	// Проверяем что сервис создался с конфигурацией по умолчанию
	if service.config == nil {
		t.Error("config не должен быть nil даже для несуществующего файла")
	}
}

// Тестируем метод IsEnabled
func TestOneCServiceIsEnabled(t *testing.T) {
	configPath := createTestConfig(t)
	defer os.Remove(configPath)

	service, err := NewOneCService(configPath)
	if err != nil {
		t.Fatalf("Не удалось создать сервис: %v", err)
	}

	// Сервис должен быть включен согласно тестовой конфигурации
	// Но может быть отключен из-за недоступности подключения к 1С
	enabled := service.IsEnabled()

	// Проверяем что метод работает без ошибок
	if enabled && service.client == nil {
		t.Error("Если сервис включен, client не должен быть nil")
	}
}

// Тестируем структуру IntegrationResult
func TestIntegrationResultCreation(t *testing.T) {
	result := IntegrationResult{
		Success:      true,
		DocumentID:   "doc-123",
		OneCRef:      "ref-456",
		ErrorMessage: "",
	}

	if !result.Success {
		t.Error("Success должен быть true")
	}

	if result.DocumentID != "doc-123" {
		t.Errorf("DocumentID не совпадает: получен %s", result.DocumentID)
	}

	if result.OneCRef != "ref-456" {
		t.Errorf("OneCRef не совпадает: получен %s", result.OneCRef)
	}
}

// Тестируем структуру IntegrationResult с ошибкой
func TestIntegrationResultError(t *testing.T) {
	result := IntegrationResult{
		Success:      false,
		DocumentID:   "doc-123",
		OneCRef:      "",
		ErrorMessage: "Connection timeout",
	}

	if result.Success {
		t.Error("Success должен быть false при ошибке")
	}

	if result.ErrorMessage == "" {
		t.Error("ErrorMessage не должен быть пустым при ошибке")
	}

	if result.OneCRef != "" {
		t.Error("OneCRef должен быть пустым при ошибке")
	}
}

// Тестируем DocumentTypeConfig
func TestDocumentTypeConfig(t *testing.T) {
	config := DocumentTypeConfig{
		TargetObject: "Document.Invoice",
		Fields: map[string]string{
			"amount": "Amount",
			"date":   "DocumentDate",
			"vendor": "Vendor",
		},
	}

	if config.TargetObject != "Document.Invoice" {
		t.Errorf("TargetObject не совпадает: получен %s", config.TargetObject)
	}

	if len(config.Fields) != 3 {
		t.Errorf("Ожидалось 3 поля, получено %d", len(config.Fields))
	}

	if config.Fields["amount"] != "Amount" {
		t.Errorf("Поле amount не совпадает: получено %s", config.Fields["amount"])
	}
}

// Тестируем процесс обработки ответа от n8n с отключенной интеграцией
func TestProcessN8nResponseDisabled(t *testing.T) {
	configPath := createTestConfig(t)
	defer os.Remove(configPath)

	service, err := NewOneCService(configPath)
	if err != nil {
		t.Fatalf("Не удалось создать сервис: %v", err)
	}

	// Принудительно отключаем интеграцию
	service.enabled = false

	n8nData := map[string]interface{}{
		"id":     "test-id",
		"text":   "Тестовый документ",
		"status": "completed",
	}

	result, err := service.ProcessN8nResponse(n8nData)
	if err != nil {
		t.Errorf("ProcessN8nResponse не должен возвращать ошибку для отключенной интеграции: %v", err)
	}

	if result == nil {
		t.Error("ProcessN8nResponse не должен возвращать nil")
	}

	if !result.Success {
		t.Error("Для отключенной интеграции Success должен быть true")
	}
}

// Benchmark для создания сервиса
func BenchmarkNewOneCService(b *testing.B) {
	// Создаём конфигурацию один раз
	config := IntegrationConfig{
		OneCConfig: OneCConfig{
			BaseURL:  "http://localhost:8080",
			Username: "testuser",
			Password: "testpass",
			Timeout:  30,
		},
		Enabled:  false, // отключаем для бенчмарка
		AutoSend: true,
	}

	configData, _ := json.MarshalIndent(config, "", "  ")
	tmpFile, _ := os.CreateTemp("", "onec_bench_config_*.json")
	tmpFile.Write(configData)
	tmpFile.Close()
	configPath := tmpFile.Name()

	defer os.Remove(configPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service, _ := NewOneCService(configPath)
		_ = service
	}
}

// Benchmark для ProcessN8nResponse
func BenchmarkProcessN8nResponse(b *testing.B) {
	configPath := createTestConfigForBench(b)
	defer os.Remove(configPath)

	service, err := NewOneCService(configPath)
	if err != nil {
		b.Fatalf("Не удалось создать сервис: %v", err)
	}

	// Отключаем интеграцию для стабильного бенчмарка
	service.enabled = false

	n8nData := map[string]interface{}{
		"id":     "bench-test-id",
		"text":   "Бенчмарк тестовый документ",
		"status": "completed",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ProcessN8nResponse(n8nData)
	}
}

// Вспомогательная функция для создания конфигурации в бенчмарке
func createTestConfigForBench(b *testing.B) string {
	config := IntegrationConfig{
		OneCConfig: OneCConfig{
			BaseURL:  "http://localhost:8080",
			Username: "testuser",
			Password: "testpass",
			Timeout:  30,
		},
		Enabled:  false, // отключено для тестов
		AutoSend: true,
		Mapping: map[string]DocumentTypeConfig{
			"invoice": {
				TargetObject: "Document.IncomingInvoice",
				Fields: map[string]string{
					"amount": "Amount",
					"vendor": "Vendor",
				},
			},
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		b.Fatalf("Не удалось сериализовать конфигурацию: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "onec_test_config_*.json")
	if err != nil {
		b.Fatalf("Не удалось создать временный файл: %v", err)
	}

	if _, err := tmpFile.Write(configData); err != nil {
		b.Fatalf("Не удалось записать конфигурацию: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}
