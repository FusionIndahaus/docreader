package main

import (
	"net/http"
	"strings"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "document-ai/docs"
)

func setupRoutes() {
	// Static files with guard for protected pages
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Защищаем главную страницу (index.html)
		if path == "/static/" || path == "/static/index.html" || strings.HasSuffix(path, "/index.html") {
			if c, err := r.Cookie("admin_session"); !(err == nil && c.Value != "" && adminSessions[c.Value] != "") {
				if c2, err2 := r.Cookie("user_session"); !(err2 == nil && c2.Value != "" && getUserEmail(c2.Value) != "") {
					http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
					return
				}
			}
		}

		// Защищаем все HTML-файлы в /static/user/ кроме login.html
		if strings.HasPrefix(path, "/static/user/") && strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, "login.html") {
			if c, err := r.Cookie("admin_session"); !(err == nil && c.Value != "" && adminSessions[c.Value] != "") {
				if c2, err2 := r.Cookie("user_session"); !(err2 == nil && c2.Value != "" && getUserEmail(c2.Value) != "") {
					http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
					return
				}
			}
		}

		fs.ServeHTTP(w, r)
	})

	// Admin static UI under /admin/
	adminFS := http.FileServer(http.Dir(staticDir + "/admin"))
	http.Handle("/admin/", http.StripPrefix("/admin/", adminFS))

	// Главная доступна только авторизованным (админ или пользователь)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// проверим admin_session или user_session
		if c, err := r.Cookie("admin_session"); err == nil && c.Value != "" && adminSessions[c.Value] != "" {
			handleHome(w, r)
			return
		}
		if c, err := r.Cookie("user_session"); err == nil && c.Value != "" && getUserEmail(c.Value) != "" {
			handleHome(w, r)
			return
		}
		http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
	})
	// API endpoints - требуют авторизации пользователя (или админа)
	http.HandleFunc("/upload", requireUserOrAdmin(handleFileUpload))
	http.HandleFunc("/results", handleGetResults) // уже защищен внутри функции
	http.HandleFunc("/events", handleEvents)      // проверка внутри функции

	// Webhook от n8n - доступен без авторизации (внешний сервис)
	http.HandleFunc("/webhook", handleN8nWebhook)

	// Health check - доступен всем
	http.HandleFunc("/health", handleHealthCheck)

	// Download - требует авторизацию
	http.HandleFunc("/download", requireUserOrAdmin(handleDownload))
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	// Admin API
	http.HandleFunc("/admin/login", handleAdminLogin)
	http.HandleFunc("/admin/logout", handleAdminLogout)
	http.HandleFunc("/admin/customers", requireAdmin(handleAdminCustomersList))
	http.HandleFunc("/admin/customers/create", requireAdmin(handleAdminCustomersCreate))
	http.HandleFunc("/admin/customers/update", requireAdmin(handleAdminCustomersUpdate))
	http.HandleFunc("/admin/customers/delete", requireAdmin(handleAdminCustomersDelete))
	http.HandleFunc("/admin/customers/regenerate_password", requireAdmin(handleAdminCustomersRegeneratePassword))
	http.HandleFunc("/admin/subscriptions/create", requireAdmin(handleAdminSubscriptionsCreate))

	// User auth
	http.HandleFunc("/user/login", handleUserLogin)
	http.HandleFunc("/user/logout", handleUserLogout)

	// User dashboard
	http.HandleFunc("/user/dashboard", requireUser(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticDir+"/user/dashboard.html")
	}))
	http.HandleFunc("/user/profile", requireUser(handleUserProfile))
	http.HandleFunc("/user/change-password", requireUser(handleUserChangePassword))
	http.HandleFunc("/user/settings", requireUser(handleUserSettings))
	http.HandleFunc("/user/subscription", requireUser(handleUserSubscription))
	http.HandleFunc("/user/history", requireUser(handleUserHistory))
	http.HandleFunc("/user/usage-stats", requireUser(handleUserUsageStats))
}
