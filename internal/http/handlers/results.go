package handlers

import "net/http"

type ResultsDeps struct {
	RequireUser      func(http.HandlerFunc) http.HandlerFunc
	SendJSONResponse func(http.ResponseWriter, interface{})
	SendJSONError    func(http.ResponseWriter, string, int)
	GetUserEmail     func(string) string
	GetUserResults   func(email string) interface{}
}

func RegisterResultsRoutes(mux *http.ServeMux, d ResultsDeps) {
	mux.HandleFunc("/results", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		email := getRequestUserEmail(r, d.GetUserEmail)
		if email == "" {
			d.SendJSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		results := d.GetUserResults(email)
		d.SendJSONResponse(w, map[string]interface{}{"status": "success", "data": results})
	}))
}
