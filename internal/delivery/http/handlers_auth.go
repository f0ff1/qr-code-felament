package http

import (
	"encoding/json"
	"net/http"

	"filamenttracker/internal/bootstrap"
	userdomain "filamenttracker/internal/domain/user"
	authusecase "filamenttracker/internal/usecase/auth"

	"github.com/google/uuid"
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
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		sess, err := app.Auth.Login(r.Context(), input.Username, input.Password)
		if err != nil {
			writeAPIError(w, err, http.StatusUnauthorized)
			return
		}
		setSessionCookie(w, app, sess.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated":  true,
			"username":       sess.Username,
			"role":           sess.Role,
			"active_site_id": sess.ActiveSiteID.String(),
			"site_ids":       uuidStrings(sess.SiteIDs),
			"is_admin":       sess.Role == string(userdomain.RoleAdmin),
		})
	})

	mux.HandleFunc("/api/auth/forgot-password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		_ = app.Auth.RequestPasswordReset(r.Context(), input.Username)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"message": "Если пользователь существует, администратор получит заявку на сброс пароля",
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
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false, "auth_enabled": true})
			return
		}
		sess, err := app.Auth.Authenticate(cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": false, "auth_enabled": true})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated":  true,
			"auth_enabled":   true,
			"username":       sess.Username,
			"role":           sess.Role,
			"active_site_id": sess.ActiveSiteID.String(),
			"site_ids":       uuidStrings(sess.SiteIDs),
			"is_admin":       sess.Role == string(userdomain.RoleAdmin),
		})
	})

	mux.HandleFunc("/api/auth/active-site", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sess, ok := sessionFrom(r.Context())
		if !ok {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		var input struct {
			SiteID string `json:"site_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		siteID, err := uuid.Parse(stringsTrim(input.SiteID))
		if err != nil {
			jsonError(w, "invalid site id", http.StatusBadRequest)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		if err := app.Auth.SetActiveSite(cookie.Value, siteID, sess); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"active_site_id": siteID.String()})
	})
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
