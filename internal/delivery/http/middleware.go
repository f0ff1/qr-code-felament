package http

import (
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	authusecase "filamenttracker/internal/usecase/auth"
)

const sessionCookieName = "ft_session"

func withMaxBytes(next http.Handler, maxBytes int64) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func withAuth(app *bootstrap.App, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresAuth(app, r) {
			next.ServeHTTP(w, r)
			return
		}
		if app.Auth == nil || !app.Auth.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		if _, err := app.Auth.Authenticate(cookie.Value); err != nil {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requiresAuth(app *bootstrap.App, r *http.Request) bool {
	path := r.URL.Path
	if path == "/events" {
		return app.Auth != nil && app.Auth.Enabled()
	}
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	switch path {
	case "/api/health", "/api/auth/login", "/api/auth/me", "/api/config":
		return false
	default:
		return true
	}
}

func setSessionCookie(w http.ResponseWriter, app *bootstrap.App, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.Auth != nil && app.Auth.CookieSecure(),
		MaxAge:   app.Auth.SessionTTL(),
	})
}

func clearSessionCookie(w http.ResponseWriter, app *bootstrap.App) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   app.Auth != nil && app.Auth.CookieSecure(),
		MaxAge:   -1,
	})
}
