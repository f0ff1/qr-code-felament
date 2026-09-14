package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/config"
	notificationusecase "filamenttracker/internal/usecase/notification"
)

type notifyBridge struct {
	svc *notificationusecase.Service
}

func (n notifyBridge) Publish(eventType, message string, payload map[string]any) {
	if n.svc == nil {
		return
	}
	n.svc.Publish(eventType, message, payload)
}

func NewRouter() http.Handler {
	cfg := config.Load()
	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}
	app.Start(context.Background())
	return NewServer(app)
}

func NewServer(app *bootstrap.App) http.Handler {
	mux := http.NewServeMux()

	registerAuthAndConfigRoutes(mux, app)
	registerAdminRoutes(mux, app)
	registerBambuRoutes(mux, app)
	registerSpoolRoutes(mux, app)
	registerPrinterRoutes(mux, app)
	registerProductRoutes(mux, app)
	registerPrintJobRoutes(mux, app)
	registerInventoryRoutes(mux, app)
	registerPublicRoutes(mux, app)
	registerHealthAndStaticRoutes(mux, app)

	var handler http.Handler = mux
	handler = withAuth(app, handler)
	handler = withMaxBytes(handler, app.Config.HTTPMaxBodyBytes)
	return handler
}

func registerHealthAndStaticRoutes(mux *http.ServeMux, app *bootstrap.App) {
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "memory"
		if app.Runtime != nil && app.Runtime.DB != nil {
			dbStatus = "postgres"
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := app.Runtime.DB.PingContext(ctx); err != nil {
				jsonError(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "db": dbStatus})
	})

	webRoot := filepath.Join(".", "web")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(webRoot, "assets")))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		indexPath := filepath.Join(webRoot, "index.html")
		content, err := os.ReadFile(indexPath)
		if err != nil {
			jsonError(w, "index not found", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(content)
	})
}
