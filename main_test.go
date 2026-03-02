package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func BenchmarkGenerateSimpleID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSimpleID()
	}
}

func BenchmarkTruncateString(b *testing.B) {
	longString := "Это очень длинная строка для тестирования производительности функции обрезки"
	for i := 0; i < b.N; i++ {
		truncateString(longString, 20)
	}
}

func BenchmarkIsValidFileType(b *testing.B) {
	testFiles := []string{"test.pdf", "image.jpg", "doc.docx", "archive.zip"}
	for i := 0; i < b.N; i++ {
		for _, filename := range testFiles {
			isValidFileType(filename)
		}
	}
}
