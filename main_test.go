package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Тестируем функцию проверки типов файлов
func TestIsValidFileType(t *testing.T) {
	testCases := []struct {
		filename string
		expected bool
	}{
		{"document.pdf", true},
		{"image.jpg", true},
		{"image.jpeg", true},
		{"image.png", true},
		{"document.doc", false},
		{"archive.zip", false},
		{"", false},
		{"noextension", false},
		{"test.PDF", true}, // проверяю регистр
		{"test.JPG", true},
		{"file.txt", false},
	}

	for _, tc := range testCases {
		result := isValidFileType(tc.filename)
		if result != tc.expected {
			t.Errorf("isValidFileType('%s') = %v, ожидалось %v", tc.filename, result, tc.expected)
		}
	}
}

// Тестируем функцию генерации простого ID
func TestGenerateSimpleID(t *testing.T) {
	id1 := generateSimpleID()
	time.Sleep(1 * time.Millisecond) // небольшая пауза для уникальности
	id2 := generateSimpleID()

	if id1 == id2 {
		t.Error("generateSimpleID должен генерировать уникальные ID")
	}

	if !strings.HasPrefix(id1, "res_") {
		t.Errorf("ID должен начинаться с 'res_', получен: %s", id1)
	}
}

// Тестируем функцию обрезки строк
func TestTruncateString(t *testing.T) {
	testCases := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Hello World", 5, "Hello..."},
		{"Short", 10, "Short"},
		{"", 5, ""},
		{"Exactly", 7, "Exactly"},
		{"Test", 18, "Test"},
	}

	for _, tc := range testCases {
		result := truncateString(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateString('%s', %d) = '%s', ожидалось '%s'",
				tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

// Тестируем функцию отправки JSON ответов
func TestSendJSONResponse(t *testing.T) {
	rr := httptest.NewRecorder()

	testData := APIResponse{
		Status:  "success",
		Message: "Тестовое сообщение",
		Data:    map[string]string{"key": "value"},
	}

	sendJSONResponse(rr, testData)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("sendJSONResponse вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Неправильный Content-Type: получен %s, ожидался application/json", contentType)
	}
}

// Тестируем функцию отправки JSON ошибок
func TestSendJSONError(t *testing.T) {
	rr := httptest.NewRecorder()

	sendJSONError(rr, "Тестовая ошибка", http.StatusBadRequest)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("sendJSONError вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusBadRequest)
	}

	var response APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Не удалось распарсить JSON ответ: %v", err)
	}

	if response.Status != "error" {
		t.Errorf("Неожиданный статус: получен %v, ожидался 'error'", response.Status)
	}

	if response.Message != "Тестовая ошибка" {
		t.Errorf("Неожиданное сообщение: получено '%v', ожидалось 'Тестовая ошибка'", response.Message)
	}
}

// Тестируем health check endpoint
func TestHandleHealthCheck(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleHealthCheck)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusOK)
	}

	var response APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Не удалось распарсить JSON ответ: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Неожиданный статус: получен %v, ожидался 'success'", response.Status)
	}
}

// Тестируем webhook с неправильным методом
func TestHandleN8nWebhookWrongMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/webhook", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleN8nWebhook)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler должен вернуть %v для GET запроса, получен %v",
			http.StatusMethodNotAllowed, status)
	}
}

// Тестируем webhook с валидными данными
func TestHandleN8nWebhookValid(t *testing.T) {
	// Сохраняем и инициализируем переменные
	responsesMutex.Lock()
	originalResponses := responses
	originalMaxResponses := maxResponses
	responses = []ProcessingResponse{}
	maxResponses = 20 // устанавливаем достаточный лимит
	responsesMutex.Unlock()

	// Восстанавливаем после теста
	defer func() {
		responsesMutex.Lock()
		responses = originalResponses
		maxResponses = originalMaxResponses
		responsesMutex.Unlock()
	}()

	testData := map[string]interface{}{
		"id":     "test-webhook-id",
		"text":   "Обработанный текст документа",
		"status": "completed",
	}

	jsonData, _ := json.Marshal(testData)

	req, err := http.NewRequest("POST", "/webhook", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleN8nWebhook)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusOK)
	}

	// Проверяем, что ответ добавился в responses
	responsesMutex.RLock()
	responsesCount := len(responses)
	responsesMutex.RUnlock()

	if responsesCount != 1 {
		t.Errorf("Ожидался 1 ответ в responses, получено %d", responsesCount)
	}
}

// Тестируем обработку GET запросов к результатам
func TestHandleGetResults(t *testing.T) {
	// Сохраняем оригинальные данные
	responsesMutex.Lock()
	originalResponses := responses
	responses = []ProcessingResponse{
		{
			ID:        "test-1",
			Text:      "Тестовый документ",
			Timestamp: time.Now(),
			Status:    "completed",
		},
	}
	responsesMutex.Unlock()

	// Восстанавливаем после теста
	defer func() {
		responsesMutex.Lock()
		responses = originalResponses
		responsesMutex.Unlock()
	}()

	req, err := http.NewRequest("GET", "/results", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleGetResults)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusOK)
	}

	var response APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Не удалось распарсить JSON ответ: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Неожиданный статус: получен %v, ожидался 'success'", response.Status)
	}
}

// Тестируем статус 1С когда сервис не настроен
func TestHandleOneCStatusNotConfigured(t *testing.T) {
	// Временно обнуляем oneCService
	originalService := oneCService
	oneCService = nil
	defer func() { oneCService = originalService }()

	req, err := http.NewRequest("GET", "/onec/status", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleOneCStatus)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler вернул неправильный статус код: получен %v, ожидался %v",
			status, http.StatusOK)
	}

	var response APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Не удалось распарсить JSON ответ: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Неожиданный статус: получен %v, ожидался 'success'", response.Status)
	}
}

// Benchmark для функции генерации ID
func BenchmarkGenerateSimpleID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSimpleID()
	}
}

// Benchmark для функции обрезки строк
func BenchmarkTruncateString(b *testing.B) {
	longString := "Это очень длинная строка для тестирования производительности функции обрезки"
	for i := 0; i < b.N; i++ {
		truncateString(longString, 20)
	}
}

// Benchmark для проверки типов файлов
func BenchmarkIsValidFileType(b *testing.B) {
	testFiles := []string{"test.pdf", "image.jpg", "doc.docx", "archive.zip"}
	for i := 0; i < b.N; i++ {
		for _, filename := range testFiles {
			isValidFileType(filename)
		}
	}
}
