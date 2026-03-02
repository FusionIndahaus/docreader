package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type DownloadDeps struct {
	RequireUserOrAdmin func(http.HandlerFunc) http.HandlerFunc
	SendJSONError      func(http.ResponseWriter, string, int)
	GetEnv             func(string, string) string
}

func RegisterDownload(mux *http.ServeMux, d DownloadDeps) {
	mux.HandleFunc("/download", d.RequireUserOrAdmin(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			d.SendJSONError(w, "Не указано имя файла", http.StatusBadRequest)
			return
		}
		if strings.Contains(name, "/") || strings.Contains(name, "\\") {
			d.SendJSONError(w, "Некорректное имя файла", http.StatusBadRequest)
			return
		}
		uploadDir := d.GetEnv("UPLOAD_DIR", "/tmp/document-ai/uploads")
		filePath := filepath.Join(uploadDir, name)
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			d.SendJSONError(w, "Файл не найден", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		f, err := os.Open(filePath)
		if err != nil {
			d.SendJSONError(w, "Не удалось открыть файл", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		_, _ = io.Copy(w, f)
	}))
}
