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
	"sync"
	"time"
)

// Конфигурация приложения
const (
	// URL вебхука n8n - сюда отправляем все файлы
	n8nWebhookURL = "https://qbitagents.app.n8n.cloud/webhook-test/d8f99a21-dc92-4dac-9746-6581ce15df8f"
	serverPort    = "8080"
	maxFileSize   = 50 << 20 // 50MB - думаю, хватит для большинства документов
	maxResponses  = 20       // храним последние 20 ответов
)

// Структуры данных
type DocumentRequest struct {
	Message  string `json:"message"`
	FileName string `json:"fileName"`
}

type ProcessingResponse struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Глобальное состояние приложения
// Да, знаю что глобальные переменные не очень, но для простого приложения сойдет
var (
	responses      []ProcessingResponse
	responsesMutex sync.RWMutex // RWMutex чтобы читать могли несколько горутин одновременно
)

func main() {
	setupRoutes()
	startServer()
}

// Настраиваем все роуты
func setupRoutes() {
	// Статические файлы (CSS, JS, картинки если будут)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Основные эндпоинты
	http.HandleFunc("/", handleHome)              // главная страница
	http.HandleFunc("/upload", handleFileUpload)  // загрузка файлов
	http.HandleFunc("/webhook", handleN8nWebhook) // прием данных от n8n
	http.HandleFunc("/results", handleGetResults) // получение результатов обработки
	http.HandleFunc("/health", handleHealthCheck) // проверка здоровья сервиса

	log.Println("🚀 Роуты настроены, готов к работе!")
}

// Запускаем веб-сервер
func startServer() {
	log.Printf("Сервер запускается на порту %s", serverPort)
	log.Printf("Откройте http://localhost:%s в браузере", serverPort)

	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatal("Не удалось запустить сервер:", err)
	}
}

// Главная страница - просто отдаем index.html
func handleHome(w http.ResponseWriter, r *http.Request) {
	// Защищаемся от попыток доступа к другим путям
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, "static/index.html")
}

// Обработка загрузки файлов - основная фишка приложения
func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST запросы, друг!", http.StatusMethodNotAllowed)
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
		sendJSONError(w, "Описание документа обязательно!", http.StatusBadRequest)
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

	log.Printf("Получен файл: %s (размер: %d байт)", header.Filename, header.Size)

	// Отправляем в n8n
	if err := sendToN8n(message, file, header.Filename); err != nil {
		log.Printf("Ошибка отправки в n8n: %v", err)
		sendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Файл %s успешно отправлен в n8n", header.Filename)
	sendJSONResponse(w, APIResponse{
		Status:  "success",
		Message: "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
	})
}

// Принимаем результаты обработки от n8n
func handleN8nWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Только POST", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Ошибка чтения webhook от n8n: %v", err)
		sendJSONError(w, "Не удалось прочитать данные", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Пытаемся распарсить JSON
	var webhookData struct {
		Text   string `json:"text"`
		Status string `json:"status"`
	}

	responseText := ""
	status := "completed"

	// Если пришел JSON - парсим его
	if err := json.Unmarshal(body, &webhookData); err == nil && webhookData.Text != "" {
		responseText = webhookData.Text
		if webhookData.Status != "" {
			status = webhookData.Status
		}
	} else {
		// Если это просто текст - используем как есть
		responseText = string(body)
	}

	if responseText == "" {
		sendJSONError(w, "Пустой ответ от n8n", http.StatusBadRequest)
		return
	}

	// Сохраняем результат
	response := ProcessingResponse{
		ID:        generateSimpleID(),
		Text:      responseText,
		Timestamp: time.Now(),
		Status:    status,
	}

	responsesMutex.Lock()
	responses = append(responses, response)
	// Ограничиваем количество сохраненных результатов
	if len(responses) > maxResponses {
		responses = responses[len(responses)-maxResponses:]
	}
	responsesMutex.Unlock()

	log.Printf("Получен результат от n8n: %s... (статус: %s)",
		truncateString(responseText, 50), status)

	sendJSONResponse(w, APIResponse{Status: "success", Message: "Результат сохранен"})
}

// Отдаем сохраненные результаты обработки
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

// Простая проверка здоровья сервиса
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	sendJSONResponse(w, APIResponse{
		Status:  "healthy",
		Message: "Сервер работает нормально",
	})
}

// Отправляем файл и сообщение в n8n
func sendToN8n(message string, file multipart.File, fileName string) error {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	// Добавляем текстовое поле
	if err := writer.WriteField("message", message); err != nil {
		return fmt.Errorf("не удалось добавить сообщение: %w", err)
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
	client := &http.Client{Timeout: 30 * time.Second} // таймаут на всякий случай
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
		log.Printf("Ошибка кодирования JSON: %v", err)
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
