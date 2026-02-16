package handlers

import (
	"document-ai/internal/docxutil"
	"document-ai/internal/pdfutil"
	"document-ai/internal/xlsxutil"
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
		userEmail := getRequestUserEmail(r, d.GetUserEmail)
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
			case ".xlsx":
				contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			case ".docx":
				contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			}
		}
		// Подсчёт страниц/листов по типу документа — лог и ответ API
		var pagesCount *int
		var sheetsCount *int
		ct := strings.ToLower(strings.TrimSpace(contentType))
		switch {
		case strings.HasPrefix(ct, "application/pdf"):
			if n, err := pdfutil.CountPagesFromBytes(fileBytes); err == nil {
				pagesCount = &n
				log.Printf("Upload: file=%q Pages Count=%d batchID=%s seq=%d", header.Filename, n, batchID, seq)
			} else {
				log.Printf("Upload: file=%q Pages Count=unknown (pdfinfo error: %v)", header.Filename, err)
			}
		case strings.Contains(ct, "spreadsheetml") || strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx"):
			if n, err := xlsxutil.SheetCountFromBytes(fileBytes); err == nil {
				sheetsCount = &n
				log.Printf("Upload: file=%q Sheets Count=%d batchID=%s seq=%d", header.Filename, n, batchID, seq)
			} else {
				log.Printf("Upload: file=%q Sheets Count=unknown (%v)", header.Filename, err)
			}
		case strings.Contains(ct, "wordprocessingml") || strings.HasSuffix(strings.ToLower(header.Filename), ".docx"):
			if n, err := docxutil.CountPagesFromBytes(fileBytes); err == nil {
				pagesCount = &n
				log.Printf("Upload: file=%q Pages Count=%d (docx) batchID=%s seq=%d", header.Filename, n, batchID, seq)
			} else {
				log.Printf("Upload: file=%q Pages Count=unknown (%v)", header.Filename, err)
			}
		}
		if err := d.StartLLMProcessing(message, header.Filename, contentType, batchID, seq, fileBytes, userEmail); err != nil {
			log.Printf("ERROR: Ошибка запуска обработки через LLM: %v", err)
			d.SendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp := map[string]interface{}{
			"status":  "success",
			"message": "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
		}
		if pagesCount != nil {
			resp["pagesCount"] = *pagesCount
		}
		if sheetsCount != nil {
			resp["sheetsCount"] = *sheetsCount
		}
		d.SendJSONResponse(w, resp)
	}))
}
