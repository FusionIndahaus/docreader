package onec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OneCClient представляет клиент для работы с 1С
type OneCClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// OneCConfig конфигурация для подключения к 1С
type OneCConfig struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Timeout  int    `json:"timeout"` // в секундах
}

// DocumentData структура данных документа для отправки в 1С
type DocumentData struct {
	ID           string                 `json:"id"`
	DocumentType string                 `json:"document_type"`
	CreatedAt    time.Time              `json:"created_at"`
	Fields       map[string]interface{} `json:"fields"`
	Metadata     DocumentMetadata       `json:"metadata"`
}

// DocumentMetadata метаданные документа
type DocumentMetadata struct {
	OriginalName string  `json:"original_name"`
	FileSize     int64   `json:"file_size"`
	ProcessedBy  string  `json:"processed_by"`
	Confidence   float64 `json:"confidence"`
}

// OneCResponse ответ от 1С
type OneCResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Reference string `json:"reference,omitempty"` // Ссылка на созданный объект в 1С
	Error     string `json:"error,omitempty"`
}

// NewOneCClient создает новый клиент для работы с 1С
func NewOneCClient(config OneCConfig) *OneCClient {
	timeout := 30 // по умолчанию 30 секунд
	if config.Timeout > 0 {
		timeout = config.Timeout
	}

	return &OneCClient{
		BaseURL:  config.BaseURL,
		Username: config.Username,
		Password: config.Password,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// SendDocument отправляет данные документа в 1С
func (c *OneCClient) SendDocument(data DocumentData) (*OneCResponse, error) {
	// Сериализуем данные в JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации данных: %w", err)
	}

	// Создаем HTTP запрос
	url := fmt.Sprintf("%s/hs/DocumentAI/CreateDocument", c.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Username, c.Password)

	// Отправляем запрос
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Парсим JSON ответ
	var oneCResponse OneCResponse
	if err := json.Unmarshal(body, &oneCResponse); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа 1С: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &oneCResponse, fmt.Errorf("1С вернула ошибку %d: %s", resp.StatusCode, oneCResponse.Error)
	}

	return &oneCResponse, nil
}

// TestConnection проверяет соединение с 1С
func (c *OneCClient) TestConnection() error {
	url := fmt.Sprintf("%s/hs/DocumentAI/Test", c.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания тестового запроса: %w", err)
	}

	req.SetBasicAuth(c.Username, c.Password)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка тестового соединения: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("1С недоступна, статус: %d", resp.StatusCode)
	}

	return nil
}

// ParseN8nResponse парсит ответ от n8n и преобразует в структуру для 1С
func ParseN8nResponse(n8nData map[string]interface{}) DocumentData {
	// Извлекаем основные поля
	documentType := getStringValue(n8nData, "document_type", "unknown")

	// Создаем карту полей для 1С
	fields := make(map[string]interface{})

	// Парсим различные типы данных, которые может вернуть n8n
	if extractedData, ok := n8nData["extracted_data"].(map[string]interface{}); ok {
		// Копируем все извлеченные поля
		for key, value := range extractedData {
			fields[key] = value
		}
	}

	// Добавляем дополнительные поля если есть
	if dates, ok := n8nData["dates"]; ok {
		fields["dates"] = dates
	}
	if amounts, ok := n8nData["amounts"]; ok {
		fields["amounts"] = amounts
	}
	if contacts, ok := n8nData["contacts"]; ok {
		fields["contacts"] = contacts
	}

	// Создаем метаданные
	metadata := DocumentMetadata{
		OriginalName: getStringValue(n8nData, "original_name", "unknown"),
		FileSize:     getInt64Value(n8nData, "file_size", 0),
		ProcessedBy:  "Document AI",
		Confidence:   getFloat64Value(n8nData, "confidence", 0.0),
	}

	return DocumentData{
		ID:           getStringValue(n8nData, "id", generateID()),
		DocumentType: documentType,
		CreatedAt:    time.Now(),
		Fields:       fields,
		Metadata:     metadata,
	}
}

// Вспомогательные функции для безопасного извлечения значений
func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func getInt64Value(data map[string]interface{}, key string, defaultValue int64) int64 {
	if value, ok := data[key].(float64); ok {
		return int64(value)
	}
	if value, ok := data[key].(int64); ok {
		return value
	}
	return defaultValue
}

func getFloat64Value(data map[string]interface{}, key string, defaultValue float64) float64 {
	if value, ok := data[key].(float64); ok {
		return value
	}
	return defaultValue
}

// generateID генерирует простой ID для документа
func generateID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}
