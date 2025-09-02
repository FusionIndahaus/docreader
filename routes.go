package main

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "document-ai/docs"
)

func setupRoutes() {
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/upload", handleFileUpload)
	http.HandleFunc("/webhook", handleN8nWebhook)
	http.HandleFunc("/results", handleGetResults)
	http.HandleFunc("/events", handleEvents)
	http.HandleFunc("/health", handleHealthCheck)
	http.HandleFunc("/download", handleDownload)
	http.Handle("/swagger/", httpSwagger.WrapHandler)
}
