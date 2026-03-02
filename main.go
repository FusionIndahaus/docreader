// @title Document AI API
// @version 1.0
// @description API для загрузки и обработки документов через n8n
// @host 45.82.153.200
// @BasePath /

package main

import (
	integrations "document-ai/internal/adapters/integrations"
	llmopenrouter "document-ai/internal/adapters/llm/openrouter"
	domain "document-ai/internal/domain"
	handlers "document-ai/internal/http/handlers"
	authmw "document-ai/internal/http/middleware"
	approuter "document-ai/internal/http/router"
	dbpkg "document-ai/internal/infra/db"
	eventbus "document-ai/internal/infra/events"
	sess "document-ai/internal/infra/session"
	"document-ai/internal/usecase"
	"net/http"
	"strings"
)

var appUsecase usecase.Service

func main() {
	initEnvVariables()
	handlers.SetCookieConfig(cookieSecure, cookieSameSite)
	if d, err := dbpkg.Init(dbDSN); err != nil {
		println("WARNING: DB connect failed:", err.Error())
	} else {
		db = d
	}
	defer dbpkg.Close(db)
	// Routes
	bus := eventbus.NewBus(&responses, &responsesMutex, &subscribers, &subscribersMux)
	sessionStore := sess.NewStore(&userSessions, &userSessionsMux)
	auth := authmw.Auth{
		GetUserEmail:  sessionStore.GetUserEmail,
		AdminSessions: adminSessions,
		ServiceAPIKey: serviceAPIKey,
	}
	appro := approuter.Deps{
		StaticDir:            staticDir,
		AdminSessions:        adminSessions,
		GetUserEmail:         sessionStore.GetUserEmail,
		RequireUser:          auth.RequireUser,
		RequireUserOrAdmin:   auth.RequireUserOrAdmin,
		RequireAdmin:         auth.RequireAdmin,
		SendJSONResponse:     func(w http.ResponseWriter, v interface{}) { sendJSONResponse(w, v) },
		SendJSONError:        func(w http.ResponseWriter, msg string, code int) { sendJSONError(w, msg, code) },
		GetEnv:               func(k, def string) string { return getEnv(k, def) },
		GenerateSessionToken: func() (string, error) { return generateSessionToken() },
		HashPassword:         func(s string) (string, error) { return hashPassword(s) },
		CheckPasswordHash:    func(hash, p string) bool { return checkPasswordHash(hash, p) },
		SafeCompareStrings:   func(a, b string) bool { return safeCompareStrings(a, b) },
		AdminEmail:           adminEmail,
		AdminPassword:        adminPassword,
		DB:          db,
		MaxFileSize: maxFileSize,
		StartLLMProcessing: func(message, fileName, contentType, batchID string, seq int, fileBytes []byte, userEmail, userID string, pagesCount int) error {
			return appUsecase.ProcessDocumentAsync(message, fileName, contentType, batchID, seq, fileBytes, userEmail, userID, pagesCount)
		},
		EventsSubscribe:       bus.Subscribe,
		EventsInitialSnapshot: bus.InitialSnapshot,
		AmoRedirectURI:        amoRedirectURI,
		GetUserHistory: func(email string) interface{} {
			responsesMutex.RLock()
			defer responsesMutex.RUnlock()
			out := make([]domain.ProcessingResponse, 0, len(responses))
			for _, resp := range responses {
				if resp.UserEmail == "" {
					continue
				}
				if strings.EqualFold(resp.UserEmail, email) {
					out = append(out, resp)
				}
			}
			if len(out) > 50 {
				out = out[:50]
			}
			return out
		},
		SetUserSession:    sessionStore.SetUserSession,
		DeleteUserSession: sessionStore.DeleteUserSession,
	}
	approuter.SetupRoutes(appro)
	// Подключаем publisher к usecase: публикуем через event bus
	appUsecase = usecase.Service{
		Publisher:         bus,
		Dispatcher:        integrations.Dispatcher{DB: db, AmoRedirectURI: amoRedirectURI},
		GenerateID:        func() string { return generateSimpleID() },
		DB:                db,
		BillingServiceURL: billingServiceURL,
		LLM: llmopenrouter.Client{
			APIKey:    openRouterAPIKey,
			BaseURL:   openRouterBaseURL,
			Model:     openRouterModel,
			SiteURL:   siteURL,
			SiteTitle: siteTitle,
		},
	}
	startServer()
}
