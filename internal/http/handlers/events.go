package handlers

import (
	"fmt"
	"net/http"
	"time"
)

type EventsDeps struct {
	RequireUser        func(http.HandlerFunc) http.HandlerFunc
	GetUserEmail       func(string) string
	Subscribe          func(userEmail string) (<-chan []byte, func())
	GetInitialSnapshot func(userEmail string) [][]byte
}

func RegisterEventsRoute(mux *http.ServeMux, d EventsDeps) {
	mux.HandleFunc("/events", d.RequireUser(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		c, err := r.Cookie("user_session")
		if err != nil || c.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userEmail := d.GetUserEmail(c.Value)
		if userEmail == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// отправим начальный снэпшот
		for _, b := range d.GetInitialSnapshot(userEmail) {
			fmt.Fprintf(w, "data: %s\n\n", string(b))
		}
		flusher.Flush()

		recv, unsubscribe := d.Subscribe(userEmail)
		defer unsubscribe()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case b, ok := <-recv:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", string(b))
				flusher.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": keep-alive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
}
