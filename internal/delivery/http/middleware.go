package http

import (
	"context"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	userdomain "filamenttracker/internal/domain/user"
	"filamenttracker/internal/infrastructure/session"
	authusecase "filamenttracker/internal/usecase/auth"

	"github.com/google/uuid"
)

const sessionCookieName = "ft_session"

type ctxKey int

const sessionCtxKey ctxKey = 1

func withSession(ctx context.Context, sess session.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, sess)
}

func sessionFrom(ctx context.Context) (session.Session, bool) {
	sess, ok := ctx.Value(sessionCtxKey).(session.Session)
	return sess, ok
}

func activeSiteID(ctx context.Context) uuid.UUID {
	sess, ok := sessionFrom(ctx)
	if !ok {
		return uuid.Nil
	}
	return sess.ActiveSiteID
}

func requireAdmin(sess session.Session) bool {
	return sess.Role == string(userdomain.RoleAdmin)
}

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
		if !requiresAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		sess, err := app.Auth.Authenticate(cookie.Value)
		if err != nil {
			writeAPIError(w, authusecase.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), sess)))
	})
}

func requiresAuth(r *http.Request) bool {
	path := r.URL.Path
	if path == "/events" {
		return true
	}
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	switch path {
	case "/api/health", "/api/auth/login", "/api/auth/me", "/api/auth/forgot-password", "/api/config":
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
