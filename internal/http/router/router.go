package router

import (
	"database/sql"
	"net/http"
	"strings"

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
	StartLLMProcessing    func(message, fileName, contentType, batchID string, seq int, fileBytes []byte, userEmail string) error
	EventsSubscribe       func(userEmail string) (<-chan []byte, func())
	EventsInitialSnapshot func(email string) [][]byte

	// user deps
	GetCustomerIDByEmail func(email string) (string, error)
	AmoRedirectURI       string
	GetUserHistory       func(email string) interface{}
	SetUserSession       func(token, email string)
	DeleteUserSession    func(token string)
}

func SetupRoutes(d Deps) {
	// Static files with guard for protected pages
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(d.StaticDir)))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// protect main index
		if path == "/static/" || path == "/static/index.html" || strings.HasSuffix(path, "/index.html") {
			if c, err := r.Cookie("admin_session"); !(err == nil && c.Value != "" && d.AdminSessions[c.Value] != "") {
				if c2, err2 := r.Cookie("user_session"); !(err2 == nil && c2.Value != "" && d.GetUserEmail(c2.Value) != "") {
					http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
					return
				}
			}
		}
		// protect user html (except login)
		if strings.HasPrefix(path, "/static/user/") && strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, "login.html") {
			if c, err := r.Cookie("admin_session"); !(err == nil && c.Value != "" && d.AdminSessions[c.Value] != "") {
				if c2, err2 := r.Cookie("user_session"); !(err2 == nil && c2.Value != "" && d.GetUserEmail(c2.Value) != "") {
					http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
					return
				}
			}
		}
		fs.ServeHTTP(w, r)
	})
	// Admin static UI under /admin/
	adminFS := http.FileServer(http.Dir(d.StaticDir + "/admin"))
	http.Handle("/admin/", http.StripPrefix("/admin/", adminFS))

	// Home
	handlerspkg.RegisterHomeRoute(http.DefaultServeMux, handlerspkg.HomeDeps{
		GetUserEmail:  d.GetUserEmail,
		AdminSessions: d.AdminSessions,
		StaticDir:     d.StaticDir,
	})

	// Upload
	handlerspkg.RegisterUploadRoutes(http.DefaultServeMux, handlerspkg.UploadDeps{
		RequireUserOrAdmin: d.RequireUserOrAdmin,
		SendJSONResponse:   d.SendJSONResponse,
		SendJSONError:      d.SendJSONError,
		GetUserEmail:       d.GetUserEmail,
		MaxFileSize:        d.MaxFileSize,
		StartLLMProcessing: d.StartLLMProcessing,
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
		GetCustomerIDByEmail: d.GetCustomerIDByEmail,
		GenerateSessionToken: d.GenerateSessionToken,
		SafeCompareStrings:   d.SafeCompareStrings,
		AmoRedirectURI:       d.AmoRedirectURI,
	})
	handlerspkg.RegisterHealth(http.DefaultServeMux, d.SendJSONResponse)
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
		GetCustomerIDByEmail: d.GetCustomerIDByEmail,
		GetUserHistory:       d.GetUserHistory,
	})
	// User integrations settings (BYOA)
	handlerspkg.RegisterIntegrationUserRoutes(http.DefaultServeMux, handlerspkg.IntegrationsDeps{
		DB:                   d.DB.(*sql.DB),
		RequireUser:          d.RequireUser,
		SendJSONResponse:     d.SendJSONResponse,
		SendJSONError:        d.SendJSONError,
		GetUserEmail:         d.GetUserEmail,
		GetCustomerIDByEmail: d.GetCustomerIDByEmail,
		AmoRedirectURI:       d.AmoRedirectURI,
	})

	// User dashboard page
	http.HandleFunc("/user/dashboard", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, d.StaticDir+"/user/dashboard.html")
	}))
}
