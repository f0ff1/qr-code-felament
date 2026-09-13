package http

import (
	"encoding/json"
	"net/http"

	"filamenttracker/internal/bootstrap"
)

func registerAuthAndConfigRoutes(mux *http.ServeMux, app *bootstrap.App) {
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(app.PricingPublic())
	})

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if app.Auth == nil || !app.Auth.Enabled() {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "auth_disabled": true})
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		sess, err := app.Auth.Login(input.Username, input.Password)
		if err != nil {
			writeAPIError(w, err, http.StatusUnauthorized)
			return
		}
		setSessionCookie(w, app, sess.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated": true,
			"username":      sess.Username,
		})
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			app.Auth.Logout(cookie.Value)
		}
		clearSessionCookie(w, app)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
	})

	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if app.Auth == nil || !app.Auth.Enabled() {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"authenticated": true,
				"auth_disabled": true,
				"username":      "dev",
			})
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
			return
		}
		sess, err := app.Auth.Authenticate(cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated": true,
			"username":      sess.Username,
		})
	})
}
