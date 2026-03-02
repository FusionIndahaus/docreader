package main

import (
	"log"
	"net/http"
)

func startServer() {
	handler := withCORS(http.DefaultServeMux)
	log.Printf("========================================")
	log.Printf("🚀 Docreader API server starting on port %s", serverPort)
	log.Printf("📝 API Mode: Frontend disabled (using qbit-site)")
	log.Printf("========================================")
	if err := http.ListenAndServe(":"+serverPort, handler); err != nil {
		log.Fatal("ERROR: Не удалось запустить сервер:", err)
	}
}
