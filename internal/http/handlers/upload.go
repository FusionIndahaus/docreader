package handlers

import (
	"database/sql"
	"document-ai/internal/docxutil"
	"document-ai/internal/http/middleware"
	"document-ai/internal/pagesbalance"
	"document-ai/internal/pdfutil"
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
	// Updated signature: userID, pagesCount passed so service can deduct post-LLM.
	StartLLMProcessing func(message, fileName, contentType, batchID string, seq int, fileBytes []byte, userEmail, userID string, pagesCount int) error
	DB                 *sql.DB
}

// countPages determines the number of pages/sheets based on file type.
// Returns 0 on error or unknown type.
func countPages(fileBytes []byte, contentType, ext string) int {
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		n, err := pdfutil.CountPagesFromBytes(fileBytes)
		if err != nil {
			log.Printf("⚠️ pdfutil: %v", err)
			return 0
		}
		return n
	case contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || ext == ".docx":
		n, err := docxutil.CountPagesFromBytes(fileBytes)
		if err != nil {
			log.Printf("⚠️ docxutil: %v", err)
			return 0
		}
		return n
	case strings.HasPrefix(contentType, "image/") || ext == ".jpg" || ext == ".jpeg" || ext == ".png":
		return 1
	default:
		return 0
	}
}

// docTypeFromExt returns a short canonical doc-type label ("pdf", "docx", "jpg", "png").
func docTypeFromExt(ext, contentType string) string {
	switch {
	case ext == ".pdf" || contentType == "application/pdf":
		return "pdf"
	case ext == ".docx" || contentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case ext == ".jpg" || ext == ".jpeg" || contentType == "image/jpeg":
		return "jpg"
	case ext == ".png" || contentType == "image/png":
		return "png"
	default:
		if ext != "" {
			return strings.TrimPrefix(ext, ".")
		}
		return "unknown"
	}
}

func RegisterUploadRoutes(mux *http.ServeMux, d UploadDeps) {
	mux.HandleFunc("/upload", d.RequireUserOrAdmin(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("📥 Upload request received from %s", r.RemoteAddr)
		if r.Method != http.MethodPost {
			d.SendJSONError(w, "Только POST запросы", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(d.MaxFileSize); err != nil {
			log.Printf("❌ ParseMultipartForm error: %v", err)
			d.SendJSONError(w, "Файл слишком большой или проблемы с формой", http.StatusBadRequest)
			return
		}
		message := strings.TrimSpace(r.FormValue("message"))
		if message == "" {
			log.Printf("❌ Message is empty")
			d.SendJSONError(w, "Описание документа обязательно", http.StatusBadRequest)
			return
		}
		_ = strings.ToLower(strings.TrimSpace(r.FormValue("outputFormat")))

		userEmail := getRequestUserEmail(r, d.GetUserEmail)
		userID := middleware.UserIDFromContext(r)
		log.Printf("👤 User email: %s, userID: %s", userEmail, userID)

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
			log.Printf("❌ FormFile error: %v", err)
			d.SendJSONError(w, "Не удалось получить файл: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			log.Printf("❌ ReadAll error: %v", err)
			d.SendJSONError(w, "Не удалось прочитать файл", http.StatusInternalServerError)
			return
		}
		log.Printf("📄 File received: %s (size: %d bytes)", header.Filename, len(fileBytes))

		contentType := header.Header.Get("Content-Type")
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if contentType == "" {
			switch ext {
			case ".pdf":
				contentType = "application/pdf"
			case ".jpg", ".jpeg":
				contentType = "image/jpeg"
			case ".png":
				contentType = "image/png"
			case ".docx":
				contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			}
		}

		pages := countPages(fileBytes, contentType, ext)
		docType := docTypeFromExt(ext, contentType)

		// ── Backend balance check ────────────────────────────────────────────
		// Only enforced when we have a userID (JWT auth) and DB available.
		if pages > 0 && userID != "" && d.DB != nil {
			balance, _, balanceErr := pagesbalance.GetBalance(d.DB, userID)
			if balanceErr != nil && balanceErr != sql.ErrNoRows {
				log.Printf("⚠️ [upload] failed to read balance for user %s: %v", userID, balanceErr)
			} else if balanceErr == nil && balance < pages {
				log.Printf("⛔ [upload] insufficient pages for user %s: need %d, have %d", userID, pages, balance)
				// Record the attempt in pages_details
				if batchID != "" {
					if _, detailErr := pagesbalance.UpsertPagesDetail(d.DB, userID, batchID, docType, pages); detailErr != nil {
						log.Printf("⚠️ [upload] failed to create pages_detail (insufficient): %v", detailErr)
					} else {
						_ = pagesbalance.UpdatePagesDetailStatus(d.DB, batchID, "insufficient_pages")
					}
				}
				d.SendJSONError(w, "Недостаточно страниц на балансе", http.StatusPaymentRequired)
				return
			}
		}

		// ── Record the request in pages_details (status=pending) ─────────────
		if d.DB != nil && userID != "" {
			if _, detailErr := pagesbalance.UpsertPagesDetail(d.DB, userID, batchID, docType, pages); detailErr != nil {
				log.Printf("⚠️ [upload] failed to upsert pages_detail: %v", detailErr)
			}
		}

		// ── Start LLM processing ─────────────────────────────────────────────
		log.Printf("🚀 Starting LLM processing: file=%s user=%s pages=%d batch=%s", header.Filename, userEmail, pages, batchID)
		if err := d.StartLLMProcessing(message, header.Filename, contentType, batchID, seq, fileBytes, userEmail, userID, pages); err != nil {
			log.Printf("ERROR: Ошибка запуска обработки через LLM: %v", err)
			d.SendJSONError(w, "Не удалось обработать документ: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ LLM processing started successfully for file: %s", header.Filename)

		d.SendJSONResponse(w, map[string]interface{}{
			"status":  "success",
			"message": "Документ отправлен на обработку! Результаты появятся ниже через несколько минут.",
			"pages":   pages,
		})
	}))
}
