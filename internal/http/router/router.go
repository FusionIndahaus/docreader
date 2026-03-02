package router

import (
	"database/sql"
	"net/http"

	handlerspkg "document-ai/internal/http/handlers"

	httpSwagger "github.com/swaggo/http-swagger"
)

type Deps struct {
	// static
	StaticDir     string
	AdminSessions map[string]string
	GetUserEmail  func(string) string

	// auth/middleware
	RequireUser        func(http.HandlerFunc) http.HandlerFunc
	RequireUserOrAdmin func(http.HandlerFunc) http.HandlerFunc
	RequireAdmin       func(http.HandlerFunc) http.HandlerFunc

	// json helpers
	SendJSONResponse func(http.ResponseWriter, interface{})
	SendJSONError    func(http.ResponseWriter, string, int)
	GetEnv           func(string, string) string

	// admin/auth helpers
	GenerateSessionToken func() (string, error)
	HashPassword         func(string) (string, error)
	CheckPasswordHash    func(string, string) bool
	SafeCompareStrings   func(string, string) bool
	AdminEmail           string
	AdminPassword        string

	// storage/DB and other deps
	DB                    interface{} // handlers accept *sql.DB; keep interface{} here to avoid import in this pkg
	MaxFileSize           int64
	StartLLMProcessing    func(message, fileName, contentType, batchID string, seq int, fileBytes []byte, userEmail, userID string, pagesCount int) error
	EventsSubscribe       func(userEmail string) (<-chan []byte, func())
	EventsInitialSnapshot func(email string) [][]byte

	// user deps
	AmoRedirectURI       string
	GetUserHistory       func(email string) interface{}
	SetUserSession       func(token, email string)
	DeleteUserSession    func(token string)
}

func SetupRoutes(d Deps) {
	// ========================================
	// FRONTEND DISABLED - API ONLY MODE
	// Frontend is served from qbit-site/frontend
	// ========================================

	// Upload
	handlerspkg.RegisterUploadRoutes(http.DefaultServeMux, handlerspkg.UploadDeps{
		RequireUserOrAdmin: d.RequireUserOrAdmin,
		SendJSONResponse:   d.SendJSONResponse,
		SendJSONError:      d.SendJSONError,
		GetUserEmail:       d.GetUserEmail,
		MaxFileSize:        d.MaxFileSize,
		StartLLMProcessing: d.StartLLMProcessing,
		DB:                 d.DB.(*sql.DB),
	})
	// Results
	handlerspkg.RegisterResultsRoutes(http.DefaultServeMux, handlerspkg.ResultsDeps{
		RequireUser:      d.RequireUser,
		SendJSONResponse: d.SendJSONResponse,
		SendJSONError:    d.SendJSONError,
		GetUserEmail:     d.GetUserEmail,
		GetUserResults: func(email string) interface{} {
			return d.GetUserHistory(email)
		},
	})
	// Events (SSE)
	handlerspkg.RegisterEventsRoute(http.DefaultServeMux, handlerspkg.EventsDeps{
		RequireUser:  d.RequireUser,
		GetUserEmail: d.GetUserEmail,
		Subscribe:    d.EventsSubscribe,
		GetInitialSnapshot: func(email string) [][]byte {
			return d.EventsInitialSnapshot(email)
		},
	})

	// Download
	handlerspkg.RegisterDownload(http.DefaultServeMux, handlerspkg.DownloadDeps{
		RequireUserOrAdmin: d.RequireUserOrAdmin,
		SendJSONError:      d.SendJSONError,
		GetEnv:             d.GetEnv,
	})
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	// Admin API
	handlerspkg.RegisterAdminRoutes(http.DefaultServeMux, handlerspkg.AdminDeps{
		DB:                   d.DB.(*sql.DB),
		RequireAdmin:         d.RequireAdmin,
		SendJSONResponse:     d.SendJSONResponse,
		SendJSONError:        d.SendJSONError,
		GenerateSessionToken: d.GenerateSessionToken,
		HashPassword:         d.HashPassword,
		CheckPasswordHash:    d.CheckPasswordHash,
		SafeCompareStrings:   d.SafeCompareStrings,
		AdminEmail:           d.AdminEmail,
		AdminPassword:        d.AdminPassword,
		AdminSessions:        d.AdminSessions,
	})
	// User amoCRM routes
	handlerspkg.RegisterAmoUserRoutes(http.DefaultServeMux, handlerspkg.Deps{
		DB:                   d.DB.(*sql.DB),
		RequireUser:          d.RequireUser,
		SendJSONResponse:     d.SendJSONResponse,
		SendJSONError:        d.SendJSONError,
		GetUserEmail:         d.GetUserEmail,
		GenerateSessionToken: d.GenerateSessionToken,
		SafeCompareStrings:   d.SafeCompareStrings,
		AmoRedirectURI:       d.AmoRedirectURI,
	})
	handlerspkg.RegisterHealth(http.DefaultServeMux, d.SendJSONResponse)
	// Internal pages-balance endpoints (called by billing service) + user balance endpoint
	handlerspkg.RegisterPagesBalanceRoutes(http.DefaultServeMux, handlerspkg.PagesBalanceDeps{
		DB:               d.DB.(*sql.DB),
		RequireUser:      d.RequireUser,
		GetUserEmail:     d.GetUserEmail,
		SendJSONResponse: d.SendJSONResponse,
		SendJSONError:    d.SendJSONError,
	})
	// User routes
	handlerspkg.RegisterUserRoutes(http.DefaultServeMux, handlerspkg.UserDeps{
		DB:                   d.DB.(*sql.DB),
		RequireUser:          d.RequireUser,
		RequireUserOrAdmin:   d.RequireUserOrAdmin,
		SendJSONResponse:     d.SendJSONResponse,
		SendJSONError:        d.SendJSONError,
		GetUserEmail:         d.GetUserEmail,
		SetUserSession:       d.SetUserSession,
		DeleteUserSession:    d.DeleteUserSession,
		GenerateSessionToken: d.GenerateSessionToken,
		HashPassword:         d.HashPassword,
		CheckPasswordHash:    d.CheckPasswordHash,
		GetUserHistory:       d.GetUserHistory,
	})
	// User integrations settings (BYOA)
	handlerspkg.RegisterIntegrationUserRoutes(http.DefaultServeMux, handlerspkg.IntegrationsDeps{
		DB:               d.DB.(*sql.DB),
		RequireUser:      d.RequireUser,
		SendJSONResponse: d.SendJSONResponse,
		SendJSONError:    d.SendJSONError,
		GetUserEmail:     d.GetUserEmail,
		AmoRedirectURI:   d.AmoRedirectURI,
	})

	// User dashboard page - DISABLED (using qbit-site frontend instead)
	// http.HandleFunc("/user/dashboard", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
	// 	http.ServeFile(w, r, d.StaticDir+"/user/dashboard.html")
	// }))
}
