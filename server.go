package main

import (
	"log"
	"net/http"
)

func startServer() {
	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatal("ERROR: Не удалось запустить сервер:", err)
	}
}
