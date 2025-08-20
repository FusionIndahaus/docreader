package main

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	n8nWebhookURL string
	serverPort    string
	maxFileSize   int64
	maxResponses  int
	staticDir     string
)

func initEnvVariables() {
	if err := godotenv.Load(); err != nil {
		log.Printf("WARNING: Не удалось загрузить переменные окружения из .env файла: %v", err)
	}

	n8nWebhookURL = os.Getenv("N8N_WEBHOOK_URL")
	if n8nWebhookURL == "" {
		n8nWebhookURL = "https://qbitagents.app.n8n.cloud/webhook-test/d8f99a21-dc92-4dac-9746-6581ce15df8f"
	}

	serverPort = os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	maxFileSizeStr := os.Getenv("MAX_FILE_SIZE_MB")
	if maxFileSizeStr == "" {
		maxFileSize = 50 << 20
	} else {
		maxFileSizeMB, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
		if err != nil {
			log.Fatalf("ERROR: Неверный формат MAX_FILE_SIZE_MB: %v", err)
		}
		maxFileSize = maxFileSizeMB << 20
	}

	maxResponsesStr := os.Getenv("MAX_RESPONSES")
	if maxResponsesStr == "" {
		maxResponses = 20
	} else {
		var err error
		maxResponses, err = strconv.Atoi(maxResponsesStr)
		if err != nil {
			log.Fatalf("ERROR: Неверный формат MAX_RESPONSES: %v", err)
		}
	}

	staticDir = os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}
}
