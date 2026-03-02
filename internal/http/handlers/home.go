package handlers

import (
	"net/http"
)

type HomeDeps struct {
	GetUserEmail  func(string) string
	AdminSessions map[string]string
	StaticDir     string
}

func RegisterHomeRoute(mux *http.ServeMux, d HomeDeps) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// проверим admin_session или user_session
		if c, err := r.Cookie("admin_session"); err == nil && c.Value != "" && d.AdminSessions[c.Value] != "" {
			serveHome(w, r, d.StaticDir)
			return
		}
		if c, err := r.Cookie("user_session"); err == nil && c.Value != "" && d.GetUserEmail(c.Value) != "" {
			serveHome(w, r, d.StaticDir)
			return
		}
		http.Redirect(w, r, "/static/user/login.html", http.StatusFound)
	})
}

func serveHome(w http.ResponseWriter, r *http.Request, staticDir string) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, staticDir+"/index.html")
}
