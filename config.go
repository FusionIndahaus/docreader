package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var (
	serverPort    string
	maxFileSize   int64
	maxResponses  int
	staticDir     string
	sessionSecret string
	dbDSN         string
	adminEmail    string
	adminPassword string
	// amoCRM
	amoClientID     string
	amoClientSecret string
	amoRedirectURI  string
	amoBaseURL      string
	// OpenRouter / Qwen
	openRouterAPIKey  string
	openRouterBaseURL string
	openRouterModel   string
	siteURL           string
	siteTitle         string
)

func initEnvVariables() {
	if err := godotenv.Load(); err != nil {
		log.Printf("WARNING: Не удалось загрузить переменные окружения из .env файла: %v", err)
	}

	amoClientID = strings.TrimSpace(os.Getenv("AMOCRM_CLIENT_ID"))
	amoClientSecret = strings.TrimSpace(os.Getenv("AMOCRM_CLIENT_SECRET"))
	amoRedirectURI = strings.TrimSpace(os.Getenv("AMOCRM_REDIRECT_URI"))
	amoBaseURL = strings.TrimSpace(os.Getenv("AMOCRM_BASE_URL")) // пример: https://anatolyanufriev.amocrm.ru
	if amoBaseURL == "" {
		// можно задать по умолчанию пустым; без него интеграция не активна
		amoBaseURL = ""
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

	sessionSecret = os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "dev-secret-change-me"
	}

	dbDSN = os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		dbDSN = "postgres://appuser:apppass@localhost:5432/appdb?sslmode=disable"
	}

	adminEmail = os.Getenv("ADMIN_EMAIL")
	adminPassword = os.Getenv("ADMIN_PASSWORD")

	// OpenRouter / Qwen
	openRouterAPIKey = strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	openRouterBaseURL = strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL"))
	if openRouterBaseURL == "" {
		openRouterBaseURL = "https://openrouter.ai/api/v1"
	}
	// Универсальная переменная модели с обратной совместимостью.
	openRouterModel = strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if openRouterModel == "" {
		openRouterModel = strings.TrimSpace(os.Getenv("QWEN_MODEL"))
	}
	if openRouterModel == "" {
		// По умолчанию — новая модель
		openRouterModel = "google/gemini-2.0-flash-001"
	}
	siteURL = strings.TrimSpace(os.Getenv("SITE_URL"))
	siteTitle = strings.TrimSpace(os.Getenv("SITE_TITLE"))
}
