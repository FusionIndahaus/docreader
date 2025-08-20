package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func startServer() {
	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatal("ERROR: Не удалось запустить сервер:", err)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, staticDir+"/index.html")
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST запросы", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		sendJSONError(w, "Файл слишком большой или проблемы с формой", http.StatusBadRequest)
		return
	}

	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		sendJSONError(w, "Описание документа обязательно", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		sendJSONError(w, "Не удалось получить файл: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !isValidFileType(header.Filename) {
		sendJSONError(w, "Поддерживаются только PDF, JPG, JPEG и PNG файлы", http.StatusBadRequest)
		return
	}

	if err := sendToN8n(message, file, header.Filename); err != nil {
		log.Printf("ERROR: Ошибка отправки в n8n: %v", err)
		sendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
	})
}

func handleN8nWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Ошибка чтения webhook от n8n: %v", err)
		sendJSONError(w, "Не удалось прочитать данные", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var webhookData map[string]interface{}
	var responseText string
	status := "completed"

	if err := json.Unmarshal(body, &webhookData); err == nil {
		if text, ok := webhookData["text"].(string); ok && text != "" {
			responseText = text
		} else if message, ok := webhookData["message"].(string); ok && message != "" {
			responseText = message
		} else {
			var parts []string

			excludeFields := map[string]bool{
				"status":        true,
				"webhookUrl":    true,
				"executionMode": true,
				"timestamp":     true,
				"id":            true,
			}

			for key, value := range webhookData {
				if excludeFields[key] {
					continue
				}

				switch v := value.(type) {
				case string:
					if v != "" {
						parts = append(parts, fmt.Sprintf("%s: %s", key, v))
					}
				case float64:
					parts = append(parts, fmt.Sprintf("%s: %.2f", key, v))
				case int:
					parts = append(parts, fmt.Sprintf("%s: %d", key, v))
				case bool:
					parts = append(parts, fmt.Sprintf("%s: %t", key, v))
				case map[string]interface{}:
					if jsonBytes, err := json.Marshal(v); err == nil {
						parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
					}
				case []interface{}:
					if jsonBytes, err := json.Marshal(v); err == nil {
						parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
					}
				default:
					parts = append(parts, fmt.Sprintf("%s: %v", key, v))
				}
			}

			if len(parts) > 0 {
				responseText = strings.Join(parts, "\n")
			}
		}

		if statusValue, ok := webhookData["status"].(string); ok && statusValue != "" {
			status = statusValue
		}
	} else {
		responseText = string(body)
		webhookData = make(map[string]interface{})
		webhookData["text"] = responseText
	}

	if responseText == "" {
		responseText = "Данные обработаны, но текст результата пуст"
		log.Printf("WARNING: Пустой текст результата, но данные получены: %v", webhookData)
	}

	response := ProcessingResponse{
		ID:        generateSimpleID(),
		Text:      responseText,
		Timestamp: time.Now(),
		Status:    status,
	}

	responsesMutex.Lock()
	responses = append(responses, response)
	if len(responses) > maxResponses {
		responses = responses[len(responses)-maxResponses:]
	}
	responsesMutex.Unlock()

	sendJSONResponse(w, APIResponse{Status: "success", Message: "Результат сохранен"})
}

func handleGetResults(w http.ResponseWriter, r *http.Request) {
	responsesMutex.RLock()
	data := make([]ProcessingResponse, len(responses))
	copy(data, responses)
	responsesMutex.RUnlock()

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   data,
	})
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"message":   "Document AI работает нормально",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "2.0.0",
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   health,
	})
}

func sendToN8n(message string, file multipart.File, fileName string) error {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if err := writer.WriteField("message", message); err != nil {
		return fmt.Errorf("не удалось добавить сообщение: %w", err)
	}

	if err := writer.WriteField("fileName", fileName); err != nil {
		return fmt.Errorf("не удалось добавить имя файла: %w", err)
	}

	if err := writer.WriteField("webhookUrl", n8nWebhookURL); err != nil {
		return fmt.Errorf("не удалось добавить webhook URL: %w", err)
	}

	if err := writer.WriteField("executionMode", "production"); err != nil {
		return fmt.Errorf("не удалось добавить режим выполнения: %w", err)
	}

	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return fmt.Errorf("не удалось создать поле для файла: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("не удалось скопировать файл: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("ошибка закрытия writer: %w", err)
	}

	req, err := http.NewRequest("POST", n8nWebhookURL, &buffer)
	if err != nil {
		return fmt.Errorf("не удалось создать запрос: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки в n8n: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("n8n вернул ошибку %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func isValidFileType(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExtensions := []string{".pdf", ".jpg", ".jpeg", ".png"}

	for _, validExt := range validExtensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

func generateSimpleID() string {
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Ошибка кодирования JSON: %v", err)
	}
}

func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Status:  "error",
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}
