package main

import (
	"log"
	"net/http"
)

func startServer() {
	handler := withCORS(http.DefaultServeMux)
	if err := http.ListenAndServe(":"+serverPort, handler); err != nil {
		log.Fatal("ERROR: Не удалось запустить сервер:", err)
	}
}
