package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
// @Param columns1c formData string false "Список колонок 1С (через запятую)"
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

	userEmail := ""
	if c, err := r.Cookie("user_session"); err == nil && c.Value != "" {
		if email := getUserEmail(c.Value); email != "" {
			userEmail = strings.ToLower(email)
		}
	}

	// Считываем список колонок 1С (через запятую) — временно не используется

	// Параметры батча (необязательные)
	batchID := strings.TrimSpace(r.FormValue("batchId"))
	seqStr := strings.TrimSpace(r.FormValue("seq"))
	seq := 0
	if seqStr != "" {
		if v, err := strconv.Atoi(seqStr); err == nil {
			seq = v
		}
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

	// Читаем файл в память для дальнейшей предобработки (OCR/текст)
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		sendJSONError(w, "Не удалось прочитать файл", http.StatusInternalServerError)
		return
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		// Грубая эвристика по расширению, если заголовка нет
		switch strings.ToLower(filepath.Ext(header.Filename)) {
		case ".pdf":
			contentType = "application/pdf"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		}
	}

	if err := startLLMProcessing(message, header.Filename, contentType, batchID, seq, fileBytes, userEmail); err != nil {
		log.Printf("ERROR: Ошибка запуска обработки через LLM: %v", err)
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

	// Проверяем: multipart/form-data или application/json
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// режим загрузки файла (любой тип)
		if err := r.ParseMultipartForm(maxFileSize); err != nil {
			sendJSONError(w, "Файл слишком большой или проблемы с формой", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			sendJSONError(w, "Не удалось получить файл: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Каталог для сохранения любых загруженных файлов
		saveDir := getEnv("UPLOAD_DIR", "/tmp/document-ai/uploads")
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			log.Printf("ERROR: не удалось создать каталог %s: %v", saveDir, err)
			sendJSONError(w, "Ошибка сервера: невозможно создать каталог", http.StatusInternalServerError)
			return
		}

		// Определяем имя файла: form name/fileName -> иначе исходное
		ext := strings.ToLower(filepath.Ext(header.Filename))
		baseName := strings.TrimSpace(r.FormValue("name"))
		if baseName == "" {
			baseName = strings.TrimSpace(r.FormValue("fileName"))
		}
		// Если baseName уже содержит расширение, совпадающее с ext — уберём его, чтобы не дублировать
		if baseName != "" {
			bl := strings.ToLower(baseName)
			if ext != "" && strings.HasSuffix(bl, ext) {
				baseName = baseName[:len(baseName)-len(ext)]
			}
		}
		if baseName == "" {
			baseName = strings.TrimSuffix(header.Filename, ext)
		}
		safeName := sanitizeFileName(baseName)
		if safeName == "" {
			safeName = "file"
		}
		finalName := safeName + ext
		savePath := filepath.Join(saveDir, finalName)

		// если имя занято — добавляем timestamp
		if _, err := os.Stat(savePath); err == nil {
			ts := time.Now().Unix()
			savePath = filepath.Join(saveDir, fmt.Sprintf("%s_%d%s", safeName, ts, ext))
		}

		out, err := os.Create(savePath)
		if err != nil {
			log.Printf("ERROR: не удалось создать файл %s: %v", savePath, err)
			sendJSONError(w, "Ошибка сервера: невозможно сохранить файл", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			log.Printf("ERROR: ошибка записи файла %s: %v", savePath, err)
			sendJSONError(w, "Ошибка при сохранении файла", http.StatusInternalServerError)
			return
		}

		// Формируем и транслируем событие для SSE-подписчиков (универсально)
		incomingMessage := strings.TrimSpace(r.FormValue("message"))
		if incomingMessage == "" {
			incomingMessage = "Файл успешно сохранён"
		}
		resp := ProcessingResponse{
			ID:        generateSimpleID(),
			Text:      incomingMessage,
			Timestamp: time.Now(),
			Status:    "completed",
			Download:  "/download?name=" + filepath.Base(savePath),
		}

		// Проксируем batchId/seq, если передал n8n
		if bid := strings.TrimSpace(r.FormValue("batchId")); bid != "" {
			resp.BatchID = bid
		}
		if seqStr := strings.TrimSpace(r.FormValue("seq")); seqStr != "" {
			if v, err := strconv.Atoi(seqStr); err == nil {
				resp.Seq = v
			}
		}

		responsesMutex.Lock()
		responses = append(responses, resp)
		if len(responses) > maxResponses {
			responses = responses[len(responses)-maxResponses:]
		}
		responsesMutex.Unlock()

		// Оповестим подписчиков SSE неблокирующе
		go func(rp ProcessingResponse) {
			subscribersMux.RLock()
			for ch := range subscribers {
				select {
				case ch <- rp:
				default:
				}
			}
			subscribersMux.RUnlock()
		}(resp)

		// Ответ клиенту с подробной информацией
		sendJSONResponse(w, APIResponse{
			Status:  "success",
			Message: "Файл успешно сохранён",
			Data: map[string]interface{}{
				"path":        savePath,
				"fileName":    header.Filename,
				"savedName":   filepath.Base(savePath),
				"download":    "/download?name=" + filepath.Base(savePath),
				"contentType": header.Header.Get("Content-Type"),
			},
		})
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

	// Проксируем batchId/seq из JSON, если есть
	if bid, ok := webhookData["batchId"].(string); ok && strings.TrimSpace(bid) != "" {
		response.BatchID = strings.TrimSpace(bid)
	}
	// seq может прийти как float64 из JSON
	if s, ok := webhookData["seq"].(float64); ok {
		response.Seq = int(s)
	} else if s2, ok := webhookData["seq"].(string); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(s2)); err == nil {
			response.Seq = v
		}
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
	// Отключаем буферизацию на стороне Nginx/Accelerated proxies
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Определяем email пользователя из сессии для фильтрации событий
	var userEmail string
	c, err := r.Cookie("user_session")
	if err == nil && c.Value != "" {
		if email := userSessions[c.Value]; email != "" {
			userEmail = strings.ToLower(email)
		}
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

	// Отправим последние результаты сразу при подключении (фильтруем по пользователю)
	if userEmail != "" {
		responsesMutex.RLock()
		snapshot := make([]ProcessingResponse, 0, len(responses))
		for _, resp := range responses {
			if resp.UserEmail != "" && strings.EqualFold(resp.UserEmail, userEmail) {
				snapshot = append(snapshot, resp)
			}
		}
		responsesMutex.RUnlock()
		for _, resp := range snapshot {
			fmt.Fprintf(w, "data: %s\n\n", toJSON(resp))
		}
		flusher.Flush()
	}

	// Heartbeat, чтобы соединение не простаивало и не обрывалось прокси
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Основной цикл отправки событий (фильтруем по пользователю)
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				return
			}
			// Отправляем только события, относящиеся к текущему пользователю
			if userEmail != "" && resp.UserEmail != "" {
				if !strings.EqualFold(resp.UserEmail, userEmail) {
					continue
				}
			}
			fmt.Fprintf(w, "data: %s\n\n", toJSON(resp))
			flusher.Flush()
		case <-ticker.C:
			// Комментарий SSE (heartbeat)
			fmt.Fprintf(w, ": keep-alive\n\n")
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
// @Description Возвращает последние результаты обработки документов пользователя
// @Tags results
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /results [get]
func handleGetResults(w http.ResponseWriter, r *http.Request) {
	// Проверяем сессию пользователя
	c, err := r.Cookie("user_session")
	if err != nil || c.Value == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	email := strings.ToLower(getUserEmail(c.Value))
	if email == "" {
		sendJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Получаем результаты из глобального массива responses, фильтруя по пользователю
	responsesMutex.RLock()
	userResults := make([]ProcessingResponse, 0, len(responses))
	for _, resp := range responses {
		if resp.UserEmail == "" {
			continue
		}
		if strings.EqualFold(resp.UserEmail, email) {
			userResults = append(userResults, resp)
		}
	}
	responsesMutex.RUnlock()

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   userResults,
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
		"status":     "healthy",
		"message":    "Document AI работает нормально",
		"timestamp":  time.Now().Format(time.RFC3339),
		"version":    "2.0.0",
		"db_enabled": strings.TrimSpace(os.Getenv("DATABASE_URL")) != "",
	}

	sendJSONResponse(w, APIResponse{
		Status: "success",
		Data:   health,
	})
}

// handleDownload godoc
// @Summary Скачать сохранённый файл
// @Description Выдаёт файл по имени из каталога UPLOAD_DIR
// @Tags Files
// @Produce octet-stream
// @Param name query string true "Имя сохранённого файла"
// @Success 200 {file} file
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /download [get]
func handleDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		sendJSONError(w, "Не указано имя файла", http.StatusBadRequest)
		return
	}

	// Защита от path traversal
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		sendJSONError(w, "Некорректное имя файла", http.StatusBadRequest)
		return
	}

	uploadDir := getEnv("UPLOAD_DIR", "/tmp/document-ai/uploads")
	filePath := filepath.Join(uploadDir, name)

	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		sendJSONError(w, "Файл не найден", http.StatusNotFound)
		return
	}

	// Ставим заголовки для принудительного скачивания
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	f, err := os.Open(filePath)
	if err != nil {
		sendJSONError(w, "Не удалось открыть файл", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		log.Printf("ERROR: Ошибка отдачи файла %s: %v", filePath, err)
	}
}

// sendToN8n — удалено (миграция на Qwen/OpenRouter)

// startLLMProcessing запускает асинхронный запрос к модели OpenRouter и публикует результат через SSE
func startLLMProcessing(message string, fileName string, contentType string, batchID string, seq int, fileBytes []byte, userEmail string) error {
	if strings.TrimSpace(openRouterAPIKey) == "" {
		return fmt.Errorf("не задан OPENROUTER_API_KEY")
	}

	type imageURL struct {
		Url string `json:"url"`
	}
	type contentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *imageURL `json:"image_url,omitempty"`
	}
	type chatMessage struct {
		Role    string        `json:"role"`
		Content []contentPart `json:"content"`
	}
	type chatRequest struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
	}
	type choiceMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Index   int           `json:"index"`
		Message choiceMessage `json:"message"`
	}
	type chatResponse struct {
		Choices []choice `json:"choices"`
	}

	// Системная инструкция для экстракции конкретных значений (по требованию)
	systemContent := `You are an assistant for extracting specific data from documents. 
Return ONLY the exact values explicitly requested by the user. 
For each requested value, output ONLY ONE line with the most suitable result. 
Do not output all similar values you see - choose only the single most appropriate one for each category which user want to exctract. 
The output must contain raw values separated by new lines, 
without quotes, equals signs, labels, explanations, or extra commentary. 
If a requested value is missing, output the word MISSING on its own line. 
Never invent or add information beyond what the user asked for.`

	userText := fmt.Sprintf(
		"File: %s\nUser request: %s",
		truncateString(fileName, 120),
		truncateString(message, 4000),
	)

	// Подготовим мультимодальные части: текст запроса + сам файл как data URI
	userParts := []contentPart{
		{Type: "text", Text: userText},
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "image/") {
		// Изображения отправляем как data URI напрямую
		b64 := base64.StdEncoding.EncodeToString(fileBytes)
		dataURI := "data:" + ct + ";base64," + b64
		userParts = append(userParts, contentPart{Type: "image_url", ImageURL: &imageURL{Url: dataURI}})
	} else if strings.Contains(ct, "pdf") {
		// Для PDF: без OCR, растеризуем первую страницу в PNG и отправим как изображение
		if pngB64, err := rasterizePDFFirstPageToPNGBase64(fileBytes); err == nil && strings.TrimSpace(pngB64) != "" {
			dataURI := "data:image/png;base64," + pngB64
			userParts = append(userParts, contentPart{Type: "image_url", ImageURL: &imageURL{Url: dataURI}})
		}
	}

	reqBody := chatRequest{
		Model: openRouterModel,
		Messages: []chatMessage{
			{Role: "system", Content: []contentPart{{Type: "text", Text: systemContent}}},
			{Role: "user", Content: userParts},
		},
	}

	// Делаем запрос в фоне и публикуем результат
	go func(rb chatRequest, batchID string, seq int, userEmail string) {
		// Подготовим HTTP-запрос
		buf, _ := json.Marshal(rb)
		httpClient := &http.Client{Timeout: 60 * time.Second}
		endpoint := strings.TrimRight(openRouterBaseURL, "/") + "/chat/completions"
		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(buf))
		if err != nil {
			publishProcessingResult("Не удалось сформировать запрос к модели", "error", batchID, seq, "", userEmail)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+openRouterAPIKey)
		if siteURL != "" {
			req.Header.Set("HTTP-Referer", siteURL)
		}
		if siteTitle != "" {
			req.Header.Set("X-Title", siteTitle)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			publishProcessingResult("Ошибка запроса к модели: "+err.Error(), "error", batchID, seq, "", userEmail)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			publishProcessingResult(fmt.Sprintf("Модель вернула ошибку %d: %s", resp.StatusCode, truncateString(string(body), 800)), "error", batchID, seq, "", userEmail)
			return
		}

		var cr chatResponse
		if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
			publishProcessingResult("Не удалось разобрать ответ модели", "error", batchID, seq, "", userEmail)
			return
		}
		var answer string
		if len(cr.Choices) > 0 {
			answer = strings.TrimSpace(cr.Choices[0].Message.Content)
		}
		if answer == "" {
			answer = "Модель не вернула содержимое ответа"
		}

		publishProcessingResult(answer, "completed", batchID, seq, "", userEmail)
	}(reqBody, batchID, seq, userEmail)

	return nil
}

// Растеризация первой страницы PDF в PNG и возврат base64 без префикса data URI.
// Требуются утилиты из poppler (pdftoppm).
func rasterizePDFFirstPageToPNGBase64(pdfBytes []byte) (string, error) {
	f, err := os.CreateTemp("", "docai_input_*.pdf")
	if err != nil {
		return "", err
	}
	pdfPath := f.Name()
	_, werr := f.Write(pdfBytes)
	cerr := f.Close()
	if werr != nil {
		os.Remove(pdfPath)
		return "", werr
	}
	if cerr != nil {
		os.Remove(pdfPath)
		return "", cerr
	}
	defer os.Remove(pdfPath)

	outPrefix := strings.TrimSuffix(pdfPath, ".pdf")
	// -singlefile чтобы получить ровно один файл: <prefix>.png
	cmd := exec.Command("pdftoppm", "-png", "-f", "1", "-l", "1", "-singlefile", pdfPath, outPrefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out // игнорируем вывод
		return "", fmt.Errorf("pdftoppm error: %w", err)
	}
	pngPath := outPrefix + ".png"
	defer os.Remove(pngPath)
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// publishProcessingResult сохраняет результат и оповещает подписчиков SSE
func publishProcessingResult(text, status, batchID string, seq int, download string, userEmail string) {
	resp := ProcessingResponse{
		ID:        generateSimpleID(),
		Text:      text,
		Timestamp: time.Now(),
		Status:    status,
		Download:  download,
		BatchID:   batchID,
		Seq:       seq,
		UserEmail: userEmail,
	}

	responsesMutex.Lock()
	responses = append(responses, resp)
	if len(responses) > maxResponses {
		responses = responses[len(responses)-maxResponses:]
	}
	responsesMutex.Unlock()

	go func(rp ProcessingResponse) {
		subscribersMux.RLock()
		for ch := range subscribers {
			select {
			case ch <- rp:
			default:
			}
		}
		subscribersMux.RUnlock()
	}(resp)
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
