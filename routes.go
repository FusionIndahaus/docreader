package main

import "net/http"

func setupRoutes() {
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/upload", handleFileUpload)
	http.HandleFunc("/webhook", handleN8nWebhook)
	http.HandleFunc("/webhook-test", handleN8nWebhook)
	http.HandleFunc("/results", handleGetResults)
	http.HandleFunc("/health", handleHealthCheck)
}
