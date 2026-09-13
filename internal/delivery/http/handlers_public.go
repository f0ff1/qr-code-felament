package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/config"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/infrastructure/qr"
	spoolusecase "filamenttracker/internal/usecase/spool"
)

func publicBaseURL(r *http.Request) string {
	cfg := config.Load()
	host := strings.TrimSpace(r.Host)
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	return cfg.PublicOrigin(host, r.Header.Get("X-Forwarded-Proto"), r.TLS != nil)
}

func cleanPublicToken(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "/")
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimPrefix(raw, "qr/")
	return raw
}

func renderSpoolPublicPage(spoolEntity spooldomain.Spool, currentRemaining, projectedRemaining int, baseURL string) string {
	statusLabel := "достаточно"
	switch {
	case spoolEntity.Status == spooldomain.StatusInUse:
		statusLabel = "в работе"
	case currentRemaining <= 0:
		statusLabel = "закончился"
	case currentRemaining < spooldomain.LowWeightGrams:
		statusLabel = "заканчивается"
	}

	percent := 0
	if spoolEntity.InitialWeight > 0 {
		percent = currentRemaining * 100 / spoolEntity.InitialWeight
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
	}
	homeURL := strings.TrimRight(baseURL, "/") + "/"

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Катушка %s</title>
  <style>
    body { font-family: Inter, Arial, sans-serif; background: #0d1117; color: #e6edf3; margin: 0; padding: 24px; }
    .card { max-width: 720px; margin: 0 auto; background: #161b22; border: 1px solid #30363d; border-radius: 18px; padding: 24px; }
    .label { display: inline-block; font-size: 12px; letter-spacing: .08em; color: #8b949e; text-transform: uppercase; margin-bottom: 8px; }
    h1 { margin: 0 0 12px; font-size: 28px; }
    .header { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
    .home-btn { display: inline-flex; align-items: center; justify-content: center; padding: 10px 16px; border-radius: 10px; background: rgba(88, 166, 255, 0.12); color: #dbeafe; border: 1px solid rgba(88, 166, 255, 0.3); text-decoration: none; font-weight: 700; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 16px; margin-top: 20px; }
    .item { background: #0d1117; border: 1px solid #30363d; border-radius: 12px; padding: 14px; }
    .item strong { display: block; font-size: 12px; color: #8b949e; margin-bottom: 6px; }
    .badge { display: inline-block; padding: 8px 12px; border-radius: 999px; background: #1f6feb; color: white; font-weight: 700; }
    .muted { color: #8b949e; word-break: break-all; }
    .qr { text-align: center; margin-top: 20px; }
    img { width: 220px; height: 220px; border-radius: 16px; background: white; padding: 12px; }
  </style>
</head>
<body>
  <div class="card">
    <div class="header">
      <div class="label">Статистика катушки</div>
      <a class="home-btn" href="%s">На главную</a>
    </div>
    <h1>%s %s</h1>
    <div class="badge">%s</div>
    <div class="grid">
      <div class="item"><strong>Производитель</strong><span>%s</span></div>
      <div class="item"><strong>Начальный вес</strong><span>%d г</span></div>
      <div class="item"><strong>Остаток на данный момент</strong><span>%d г</span></div>
      <div class="item"><strong>Остаток в будущем</strong><span>%d г</span></div>
      <div class="item"><strong>Осталось</strong><span>%d%%</span></div>
      <div class="item"><strong>QR token</strong><span class="muted">%s</span></div>
    </div>
    <div class="qr">
      <img src="/public/spools/qr/%s" alt="QR-код катушки" />
    </div>
  </div>
</body>
</html>`,
		spoolEntity.Material,
		homeURL,
		spoolEntity.Material,
		spoolEntity.Color,
		statusLabel,
		spoolEntity.Manufacturer,
		spoolEntity.InitialWeight,
		currentRemaining,
		projectedRemaining,
		percent,
		spoolEntity.QRToken,
		spoolEntity.QRToken,
	)
}

func registerPublicRoutes(mux *http.ServeMux, app *bootstrap.App) {
	spoolService := app.SpoolService
	printJobService := app.PrintJobService

	mux.HandleFunc("/public/spools/qr/", func(w http.ResponseWriter, r *http.Request) {
		token := cleanPublicToken(strings.TrimPrefix(r.URL.Path, "/public/spools/qr/"))
		if token == "" {
			jsonError(w, "missing token", http.StatusBadRequest)
			return
		}

		spoolEntity, err := spoolService.GetByQRToken(context.Background(), token)
		if err != nil {
			jsonError(w, "spool not found", http.StatusNotFound)
			return
		}

		baseURL := publicBaseURL(r)
		qrTarget := baseURL + "/spool/" + spoolEntity.QRToken
		png, err := qr.PNG(qrTarget)
		if err != nil {
			jsonError(w, "could not generate qr code", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(png)
	})

	writePublicSpoolPage := func(w http.ResponseWriter, r *http.Request, token string) {
		token = cleanPublicToken(token)
		if token == "" {
			jsonError(w, "missing token", http.StatusBadRequest)
			return
		}

		spoolEntity, err := spoolService.GetByQRToken(context.Background(), token)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="ru"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Катушка не найдена</title></head><body style="font-family:sans-serif;background:#0d1117;color:#e6edf3;padding:32px;"><p>Катушка не найдена.</p><p><a href="/" style="color:#58a6ff">На главную</a></p></body></html>`))
			return
		}

		jobs, err := printJobService.List(context.Background())
		if err != nil {
			jobs = nil
		}
		currentRemaining := spoolusecase.LiveRemaining(spoolEntity, jobs)
		projectedRemaining := spoolEntity.CurrentWeight
		baseURL := publicBaseURL(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderSpoolPublicPage(spoolEntity, currentRemaining, projectedRemaining, baseURL)))
	}

	mux.HandleFunc("/public/spools/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/public/spools/")
		if strings.HasPrefix(token, "qr/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			token = cleanPublicToken(token)
			if token == "" {
				jsonError(w, "missing token", http.StatusBadRequest)
				return
			}
			view, err := spoolService.GetPublicView(context.Background(), token)
			if err != nil {
				jsonError(w, "spool not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(view)
			return
		}
		writePublicSpoolPage(w, r, token)
	})

	mux.HandleFunc("/spool/", func(w http.ResponseWriter, r *http.Request) {
		writePublicSpoolPage(w, r, strings.TrimPrefix(r.URL.Path, "/spool/"))
	})
}
