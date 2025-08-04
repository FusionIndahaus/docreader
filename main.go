package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"document-ai/pkg/onec"

	"github.com/joho/godotenv"
)

// Конфигурация приложения - теперь читается из переменных окружения
var (
	n8nWebhookURL  string
	serverPort     string
	maxFileSize    int64
	maxResponses   int
	staticDir      string
	onecConfigPath string
)

// Структуры данных
type DocumentRequest struct {
	Message  string `json:"message"`
	FileName string `json:"fileName"`
}

type ProcessingResponse struct {
	ID         string                  `json:"id"`
	Text       string                  `json:"text"`
	Timestamp  time.Time               `json:"timestamp"`
	Status     string                  `json:"status"`
	OneCStatus *onec.IntegrationResult `json:"onec_status,omitempty"` // Новое поле для статуса 1С
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Глобальное состояние приложения
var (
	responses      []ProcessingResponse
	responsesMutex sync.RWMutex
	oneCService    *onec.OneCService // Сервис интеграции с 1С
)

func main() {
	// Инициализируем переменные окружения
	initEnvVariables()

	// Инициализируем сервис интеграции с 1С
	initOneCIntegration()

	setupRoutes()
	startServer()
}

// Инициализация переменных окружения
func initEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("WARNING: Не удалось загрузить переменные окружения из .env файла: %v", err)
		log.Println("INFO: Используются значения по умолчанию.")
	}

	n8nWebhookURL = os.Getenv("N8N_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "https://qbitagents.app.n8n.cloud/webhook-test/d8f99a21-dc92-4dac-9746-6581ce15df8f"
		log.Printf("INFO: N8N_WEBHOOK_URL не установлен, используется дефолт: %s", n8nWebhookURL)
	}

	serverPort = os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
		log.Printf("INFO: SERVER_PORT не установлен, используется дефолт: %s", serverPort)
	}

	maxFileSizeStr := os.Getenv("MAX_FILE_SIZE_MB")
	if maxFileSizeStr == "" {
		maxFileSize = 50 << 20 // 50MB по умолчанию
		log.Printf("INFO: MAX_FILE_SIZE_MB не установлен, используется дефолт: %d MB", maxFileSize>>20)
	} else {
		maxFileSizeMB, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
		if err != nil {
			log.Fatalf("ERROR: Неверный формат MAX_FILE_SIZE_MB: %v", err)
		}
		maxFileSize = maxFileSizeMB << 20 // Преобразуем MB в байты
		log.Printf("INFO: MAX_FILE_SIZE_MB установлен: %d MB", maxFileSizeMB)
	}

	maxResponsesStr := os.Getenv("MAX_RESPONSES")
	if maxResponsesStr == "" {
		maxResponses = 20
		log.Printf("INFO: MAX_RESPONSES не установлен, используется дефолт: %d", maxResponses)
	} else {
		maxResponses, err = strconv.Atoi(maxResponsesStr)
		if err != nil {
			log.Fatalf("ERROR: Неверный формат MAX_RESPONSES: %v", err)
		}
		log.Printf("INFO: MAX_RESPONSES установлен: %d", maxResponses)
	}

	staticDir = os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
		log.Printf("INFO: STATIC_DIR не установлен, используется дефолт: %s", staticDir)
	}

	onecConfigPath = os.Getenv("ONEC_CONFIG_PATH")
	if onecConfigPath == "" {
		onecConfigPath = "config/onec.json"
		log.Printf("INFO: ONEC_CONFIG_PATH не установлен, используется дефолт: %s", onecConfigPath)
	}
}

// Инициализация интеграции с 1С
func initOneCIntegration() {
	service, err := onec.NewOneCService(onecConfigPath)
	if err != nil {
		log.Printf("WARNING: Не удалось инициализировать интеграцию с 1С: %v", err)
		log.Println("INFO: Приложение будет работать без интеграции с 1С")
		return
	}

	oneCService = service

	if oneCService.IsEnabled() {
		log.Println("INFO: Интеграция с 1С активна")
	} else {
		log.Println("INFO: Интеграция с 1С отключена в конфигурации")
	}
}

// Настраиваем все роуты
func setupRoutes() {
	// Статические файлы (CSS, JS, картинки если будут)
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Основные эндпоинты
	http.HandleFunc("/", handleHome)                   // главная страница
	http.HandleFunc("/upload", handleFileUpload)       // загрузка файлов
	http.HandleFunc("/webhook", handleN8nWebhook)      // прием данных от n8n
	http.HandleFunc("/webhook-test", handleN8nWebhook) // тестовый webhook (тот же обработчик)
	http.HandleFunc("/results", handleGetResults)      // получение результатов обработки
	http.HandleFunc("/health", handleHealthCheck)      // проверка здоровья сервиса

	// Новые эндпоинты для 1С
	http.HandleFunc("/onec/status", handleOneCStatus)   // статус интеграции с 1С
	http.HandleFunc("/onec/send", handleOneCManualSend) // ручная отправка в 1С

	log.Println("INFO: Document AI готов к работе")
}

// Запускаем веб-сервер
func startServer() {
	log.Printf("INFO: Document AI запускается на порту %s", serverPort)
	log.Printf("INFO: Откройте http://localhost:%s в браузере", serverPort)

	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatal("ERROR: Не удалось запустить сервер:", err)
	}
}

// Главная страница - просто отдаем index.html
func handleHome(w http.ResponseWriter, r *http.Request) {
	// Защищаемся от попыток доступа к другим путям
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, staticDir+"/index.html")
}

// Обработка загрузки файлов - основная фишка приложения
func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST запросы", http.StatusMethodNotAllowed)
		return
	}

	// Парсим multipart форму с лимитом размера
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		sendJSONError(w, "Файл слишком большой или проблемы с формой", http.StatusBadRequest)
		return
	}

	// Достаем данные из формы
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		sendJSONError(w, "Описание документа обязательно", http.StatusBadRequest)
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		sendJSONError(w, "Не удалось получить файл: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Проверяем тип файла по расширению
	if !isValidFileType(header.Filename) {
		sendJSONError(w, "Поддерживаются только PDF, JPG, JPEG и PNG файлы", http.StatusBadRequest)
		return
	}

	log.Printf("INFO: Получен файл: %s (размер: %d байт)", header.Filename, header.Size)

	// Отправляем в n8n
	if err := sendToN8n(message, file, header.Filename); err != nil {
		log.Printf("ERROR: Ошибка отправки в n8n: %v", err)
		sendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("INFO: Файл %s успешно отправлен в n8n", header.Filename)
	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
	})
}

// Принимаем результаты обработки от n8n
func handleN8nWebhook(w http.ResponseWriter, r *http.Request) {
	// Логируем все детали запроса
	log.Printf("INFO: === WEBHOOK ЗАПРОС ===")
	log.Printf("INFO: Метод: %s", r.Method)
	log.Printf("INFO: URL: %s", r.URL.String())
	log.Printf("INFO: Remote Address: %s", r.RemoteAddr)
	log.Printf("INFO: User-Agent: %s", r.Header.Get("User-Agent"))
	log.Printf("INFO: Content-Type: %s", r.Header.Get("Content-Type"))

	// Логируем все заголовки
	log.Printf("INFO: Заголовки запроса:")
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("INFO:   %s: %s", name, value)
		}
	}

	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Ошибка чтения webhook от n8n: %v", err)
		sendJSONError(w, "Не удалось прочитать данные", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("INFO: Получены данные от n8n (длина: %d байт): %s", len(body), string(body))

	// Пытаемся распарсить JSON
	var webhookData map[string]interface{}
	var responseText string
	status := "completed"

	// Если пришел JSON - парсим его
	if err := json.Unmarshal(body, &webhookData); err == nil {
		// Ищем стандартные текстовые поля
		if text, ok := webhookData["text"].(string); ok && text != "" {
			responseText = text
		} else if message, ok := webhookData["message"].(string); ok && message != "" {
			responseText = message
		} else {
			// Универсальная обработка всех полей JSON
			var parts []string

			// Обрабатываем все поля, кроме служебных
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

				// Обрабатываем разные типы значений
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
					// Если это объект, преобразуем в JSON
					if jsonBytes, err := json.Marshal(v); err == nil {
						parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
					}
				case []interface{}:
					// Если это массив, преобразуем в JSON
					if jsonBytes, err := json.Marshal(v); err == nil {
						parts = append(parts, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
					}
				default:
					// Для других типов просто преобразуем в строку
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
		// Если это просто текст - используем как есть
		responseText = string(body)
		webhookData = make(map[string]interface{})
		webhookData["text"] = responseText
	}

	if responseText == "" {
		responseText = "Данные обработаны, но текст результата пуст"
		log.Printf("WARNING: Пустой текст результата, но данные получены: %v", webhookData)
	}

	// Создаем результат обработки
	response := ProcessingResponse{
		ID:        generateSimpleID(),
		Text:      responseText,
		Timestamp: time.Now(),
		Status:    status,
	}

	// Попытка интеграции с 1С
	if oneCService != nil {
		log.Println("INFO: Отправляем данные в 1С...")
		oneCResult, err := oneCService.ProcessN8nResponse(webhookData)
		if err != nil {
			log.Printf("ERROR: Ошибка интеграции с 1С: %v", err)
		}
		response.OneCStatus = oneCResult
	}

	responsesMutex.Lock()
	responses = append(responses, response)
	// Ограничиваем количество сохраненных результатов
	if len(responses) > maxResponses {
		responses = responses[len(responses)-maxResponses:]
	}
	responsesMutex.Unlock()

	log.Printf("INFO: Получен результат от n8n: %s... (статус: %s)",
		truncateString(responseText, 50), status)

	sendJSONResponse(w, APIResponse{Status: "success", Message: "Результат сохранен"})
}

// Статус интеграции с 1С
func handleOneCStatus(w http.ResponseWriter, r *http.Request) {
	if oneCService == nil {
		sendJSONResponse(w, APIResponse{
			Status: "success",
			Data: map[string]interface{}{
				"enabled":    false,
				"connection": "not_configured",
				"message":    "Интеграция с 1С не настроена",
			},
		})
		return
	}

	status := oneCService.GetStatus()
	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   status,
	})
}

// Ручная отправка в 1С
func handleOneCManualSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST", http.StatusMethodNotAllowed)
		return
	}

	if oneCService == nil {
		sendJSONError(w, "Интеграция с 1С не настроена", http.StatusServiceUnavailable)
		return
	}

	// Читаем данные запроса
	var requestData struct {
		DocumentID string                 `json:"document_id"`
		Data       map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		sendJSONError(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}

	// Отправляем в 1С
	result, err := oneCService.SendManually(requestData.DocumentID, requestData.Data)
	if err != nil {
		log.Printf("ERROR: Ошибка ручной отправки в 1С: %v", err)
		sendJSONError(w, fmt.Sprintf("Ошибка отправки в 1С: %v", err), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   result,
	})
}

// Отдаем сохраненные результаты обработки
func handleGetResults(w http.ResponseWriter, r *http.Request) {
	responsesMutex.RLock()
	data := make([]ProcessingResponse, len(responses))
	copy(data, responses)
	responsesMutex.RUnlock()

	log.Printf("INFO: Отдаем %d результатов обработки", len(data))
	if len(data) > 0 {
		log.Printf("DEBUG: Последний результат: ID=%s, Text=%s, Status=%s",
			data[len(data)-1].ID,
			truncateString(data[len(data)-1].Text, 100),
			data[len(data)-1].Status)
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   data,
	})
}

// Простая проверка здоровья сервиса
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"message":   "Document AI работает нормально",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "2.0.0",
	}

	// Добавляем статус интеграции с 1С
	if oneCService != nil {
		health["onec_integration"] = oneCService.GetStatus()
	} else {
		health["onec_integration"] = map[string]string{
			"status": "not_configured",
		}
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   health,
	})
}

// Отправляем файл и сообщение в n8n
func sendToN8n(message string, file multipart.File, fileName string) error {
	// Создаем буфер для multipart данных
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	// Добавляем текстовые поля
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

	// Перематываем файл в начало на всякий случай
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	// Добавляем файл
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

	// Создаем HTTP запрос
	req, err := http.NewRequest("POST", n8nWebhookURL, &buffer)
	if err != nil {
		return fmt.Errorf("не удалось создать запрос: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Отправляем запрос
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки в n8n: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("n8n вернул ошибку %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Вспомогательные функции

// Проверяем допустимые типы файлов
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

// Генерируем простой ID для результатов
func generateSimpleID() string {
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}

// Обрезаем строку для логов
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Отправляем JSON ответ
func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Ошибка кодирования JSON: %v", err)
	}
}

// Отправляем JSON ошибку
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Status:  "error",
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}
