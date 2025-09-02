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

// handleHome godoc
// @Summary Главная страница
// @Description Отдает HTML-страницу с формой загрузки документов
// @Tags Home
// @Produce html
// @Success 200 {string} string "index.html"
// @Failure 404 {object} APIResponse "Страница не найдена"
// @Router / [get]
func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, staticDir+"/index.html")
}

// handleFileUpload godoc
// @Summary Загрузка документа
// @Description Загружает файл (PDF, JPG, JPEG, PNG) с описанием и отправляет на обработку через n8n
// @Tags Documents
// @Accept multipart/form-data
// @Produce json
// @Param message formData string true "Описание документа"
// @Param outputFormat formData string false "Формат результата" Enums(json, csv, xlsx) Default(json)
// @Param file formData file true "Файл документа"
// @Success 200 {object} APIResponse "Документ успешно отправлен на обработку"
// @Failure 400 {object} APIResponse "Ошибки валидации или проблемы с файлом"
// @Failure 405 {object} APIResponse "Метод не разрешен"
// @Failure 500 {object} APIResponse "Ошибка сервера при отправке в n8n"
// @Router /upload [post]
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

	// Читаем желаемый формат результата, по умолчанию json
	outputFormat := strings.ToLower(strings.TrimSpace(r.FormValue("outputFormat")))
	switch outputFormat {
	case "json", "csv", "xlsx":
		// валидный формат
	default:
		outputFormat = "json"
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

	if err := sendToN8n(message, file, header.Filename, outputFormat); err != nil {
		log.Printf("ERROR: Ошибка отправки в n8n: %v", err)
		sendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
	})
}

// handleN8nWebhook godoc
// @Summary Вебхук от n8n
// @Description Принимает данные обработки документа от n8n и сохраняет результат
// @Tags Webhook
// @Accept json
// @Produce json
// @Param payload body map[string]interface{} true "Данные от n8n"
// @Success 200 {object} APIResponse "Результат успешно сохранен"
// @Failure 400 {object} APIResponse "Ошибка чтения данных или JSON"
// @Failure 405 {object} APIResponse "Метод не разрешен"
// @Router /webhook [post]
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

	go func(resp ProcessingResponse) {
		subscribersMux.RLock()
		for ch := range subscribers {
			select {
			case ch <- resp:
			default:
			}
		}
		subscribersMux.RUnlock()
	}(response)

	sendJSONResponse(w, APIResponse{Status: "success", Message: "Результат сохранен"})
}

// handleEvents отправляет клиенту события результатов через SSE
//
//nolint:unused
func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan ProcessingResponse, 1)

	// Регистрируем подписчика
	subscribersMux.Lock()
	if subscribers == nil {
		subscribers = make(map[chan ProcessingResponse]struct{})
	}
	subscribers[ch] = struct{}{}
	subscribersMux.Unlock()

	// При закрытии соединения удаляем подписчика
	notify := r.Context().Done()
	go func() {
		<-notify
		subscribersMux.Lock()
		delete(subscribers, ch)
		close(ch)
		subscribersMux.Unlock()
	}()

	// Отправим последние результаты сразу при подключении
	responsesMutex.RLock()
	snapshot := make([]ProcessingResponse, len(responses))
	copy(snapshot, responses)
	responsesMutex.RUnlock()
	for _, resp := range snapshot {
		fmt.Fprintf(w, "data: %s\n\n", toJSON(resp))
	}
	flusher.Flush()

	// Основной цикл отправки событий
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", toJSON(resp))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

//nolint:unused
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// handleGetResults godoc
// @Summary Получить список результатов
// @Description Возвращает последние результаты обработки документов
// @Tags results
// @Produce json
// @Success 200 {object} APIResponse
// @Router /results [get]
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

// handleHealthCheck godoc
// @Summary Проверка состояния сервиса
// @Description Возвращает статус работы сервиса, время и версию
// @Tags Health
// @Produce json
// @Success 200 {object} APIResponse "Сервис работает нормально"
// @Router /health [get]
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

func sendToN8n(message string, file multipart.File, fileName string, outputFormat string) error {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	// Дублируем в промпт пользователя требуемый формат
	messageWithFormat := message + "\n" + "Верни данные в виде " + outputFormat
	if err := writer.WriteField("message", messageWithFormat); err != nil {
		return fmt.Errorf("не удалось добавить сообщение: %w", err)
	}

	if err := writer.WriteField("outputFormat", outputFormat); err != nil {
		return fmt.Errorf("не удалось добавить формат результата: %w", err)
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
