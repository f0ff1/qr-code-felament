package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	printerusecase "filamenttracker/internal/usecase/printer"
)

func registerBambuRoutes(mux *http.ServeMux, app *bootstrap.App) {
	printerService := app.PrinterService
	notifier := app.NotificationService
	bambuMonitor := app.Monitor

	mux.HandleFunc("/api/bambu/cloud/account", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			account, err := printerService.GetCloudAccount(context.Background())
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"linked": false})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"linked": account.Linked(),
				"email":  account.Email,
				"region": account.Region,
			})
		case http.MethodDelete:
			if !requireWritable(w, r) {
				return
			}
			if err := printerService.LogoutCloud(context.Background()); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			bambuMonitor.ResetCloudSessions()
			notifier.Publish("bambu_cloud_logout", "Вы вышли из Bambu Cloud", withSitePayload(map[string]any{}, activeSiteID(r.Context())))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"linked": false})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/bambu/cloud/resend-code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireWritable(w, r) {
			return
		}
		var input struct {
			Email  string `json:"email"`
			Region string `json:"region"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(input.Email)
		region := input.Region
		if email == "" || region == "" {
			if account, err := printerService.GetCloudAccount(context.Background()); err == nil {
				if email == "" {
					email = account.Email
				}
				if region == "" {
					region = account.Region
				}
			}
		}
		if email == "" {
			jsonError(w, "email is required", http.StatusBadRequest)
			return
		}
		if err := printerService.SendEmailCode(email, region); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"message": "Код повторно отправлен на " + email,
		})
	})

	mux.HandleFunc("/api/bambu/cloud/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireWritable(w, r) {
			return
		}
		var input struct {
			Email      string `json:"email"`
			Password   string `json:"password"`
			Region     string `json:"region"`
			VerifyCode string `json:"verifyCode"`
			Token      string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		siteID := activeSiteID(r.Context())
		result, err := printerService.SyncFromCloud(context.Background(), printerusecase.CloudSyncInput{
			SiteID:     siteID,
			Email:      input.Email,
			Password:   input.Password,
			Region:     input.Region,
			VerifyCode: input.VerifyCode,
			Token:      input.Token,
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if result.NeedsVerify {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"needs_verification": true,
				"message":            "Код отправлен на email. Введите 6-значный код и нажмите «Войти» снова.",
			})
			return
		}

		// Pull live print jobs immediately after sync.
		bambuMonitor.PollOnce(context.Background())

		printers := make([]map[string]any, 0, len(result.Printers))
		for _, p := range result.Printers {
			if !sameSite(p.SiteID, siteID) {
				continue
			}
			printers = append(printers, printerJSON(p))
		}
		devices := make([]map[string]any, 0, len(result.Devices))
		for _, d := range result.Devices {
			devices = append(devices, map[string]any{
				"serial":       d.Serial,
				"name":         d.Name,
				"model":        d.Model,
				"online":       d.Online,
				"print_status": d.PrintStatus,
			})
		}
		notifier.Publish("bambu_cloud_synced", "Принтеры синхронизированы из Bambu Cloud", withSitePayload(map[string]any{
			"count": len(printers),
		}, siteID))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"needs_verification": false,
			"printers":           printers,
			"devices":            devices,
			"count":              len(printers),
		})
	})
}
