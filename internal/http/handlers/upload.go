package handlers

import (
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type UploadDeps struct {
	RequireUserOrAdmin func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse   func(http.ResponseWriter, interface{})
	SendJSONError      func(http.ResponseWriter, string, int)
	GetUserEmail       func(string) string
	MaxFileSize        int64
	StartLLMProcessing func(message string, fileName string, contentType string, batchID string, seq int, fileBytes []byte, userEmail string) error
}

func RegisterUploadRoutes(mux *http.ServeMux, d UploadDeps) {
	// /upload (auth required)
	mux.HandleFunc("/upload", d.RequireUserOrAdmin(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			d.SendJSONError(w, "Только POST запросы", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(d.MaxFileSize); err != nil {
			d.SendJSONError(w, "Файл слишком большой или проблемы с формой", http.StatusBadRequest)
			return
		}
		message := strings.TrimSpace(r.FormValue("message"))
		if message == "" {
			d.SendJSONError(w, "Описание документа обязательно", http.StatusBadRequest)
			return
		}
		// outputFormat сейчас не влияет на поведение
		_ = strings.ToLower(strings.TrimSpace(r.FormValue("outputFormat")))
		userEmail := ""
		if c, err := r.Cookie("user_session"); err == nil && c.Value != "" {
			if email := d.GetUserEmail(c.Value); email != "" {
				userEmail = strings.ToLower(email)
			}
		}
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
			d.SendJSONError(w, "Не удалось получить файл: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			d.SendJSONError(w, "Не удалось прочитать файл", http.StatusInternalServerError)
			return
		}
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			switch strings.ToLower(filepath.Ext(header.Filename)) {
			case ".pdf":
				contentType = "application/pdf"
			case ".jpg", ".jpeg":
				contentType = "image/jpeg"
			case ".png":
				contentType = "image/png"
			}
		}
		if err := d.StartLLMProcessing(message, header.Filename, contentType, batchID, seq, fileBytes, userEmail); err != nil {
			log.Printf("ERROR: Ошибка запуска обработки через LLM: %v", err)
			d.SendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
			return
		}
		d.SendJSONResponse(w, map[string]interface{}{
			"status":  "success",
			"message": "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
		})
	}))
}
