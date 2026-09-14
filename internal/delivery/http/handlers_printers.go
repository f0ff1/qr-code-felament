package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	printerdomain "filamenttracker/internal/domain/printer"
	printerusecase "filamenttracker/internal/usecase/printer"

	"github.com/google/uuid"
)

func printerJSON(printer printerdomain.Printer) map[string]any {
	item := map[string]any{
		"id":                 printer.ID.String(),
		"name":               printer.Name,
		"model":              printer.Model,
		"status":             string(printer.Status),
		"lan_enabled":        printer.LANEnabled,
		"lan_host":           printer.LANHost,
		"lan_serial":         printer.LANSerial,
		"cloud_enabled":      printer.CloudEnabled,
		"cloud_region":       printer.CloudRegion,
		"cloud_email":        printer.CloudEmail,
		"cloud_linked":       printer.CloudLinked(),
		"connection":         string(printer.ConnectionMode()),
		"needs_verification": printer.CloudEnabled && !printer.CloudLinked(),
	}
	if printer.DefaultSpoolID != uuid.Nil {
		item["default_spool_id"] = printer.DefaultSpoolID.String()
	}
	return item
}

func registerPrinterRoutes(mux *http.ServeMux, app *bootstrap.App) {
	printerService := app.PrinterService
	notifier := app.NotificationService

	mux.HandleFunc("/api/printers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			printers, err := printerService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			siteID := activeSiteID(r.Context())
			payload := make([]map[string]any, 0, len(printers))
			for _, printer := range printers {
				if !sameSite(printer.SiteID, siteID) {
					continue
				}
				payload = append(payload, printerJSON(printer))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			if !requireWritable(w, r) {
				return
			}
			var input struct {
				Name            string `json:"name"`
				Model           string `json:"model"`
				LANHost         string `json:"lanHost"`
				LANSerial       string `json:"lanSerial"`
				LANAccessCode   string `json:"lanAccessCode"`
				LANEnabled      bool   `json:"lanEnabled"`
				CloudEnabled    bool   `json:"cloudEnabled"`
				CloudEmail      string `json:"cloudEmail"`
				CloudPassword   string `json:"cloudPassword"`
				CloudToken      string `json:"cloudToken"`
				CloudRegion     string `json:"cloudRegion"`
				CloudVerifyCode string `json:"cloudVerifyCode"`
				DefaultSpoolID  string `json:"defaultSpoolId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			createInput := printerusecase.CreateInput{
				SiteID:          activeSiteID(r.Context()),
				Name:            input.Name,
				Model:           input.Model,
				LANHost:         input.LANHost,
				LANSerial:       input.LANSerial,
				LANAccessCode:   input.LANAccessCode,
				LANEnabled:      input.LANEnabled,
				CloudEnabled:    input.CloudEnabled,
				CloudEmail:      input.CloudEmail,
				CloudPassword:   input.CloudPassword,
				CloudToken:      input.CloudToken,
				CloudRegion:     input.CloudRegion,
				CloudVerifyCode: input.CloudVerifyCode,
			}
			if input.DefaultSpoolID != "" {
				spoolID, err := uuid.Parse(input.DefaultSpoolID)
				if err != nil {
					jsonError(w, "invalid default spool id", http.StatusBadRequest)
					return
				}
				createInput.DefaultSpoolID = spoolID
			}
			result, err := printerService.Create(context.Background(), createInput)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("printer_created", "Printer registered", withSitePayload(map[string]any{"printer_id": result.ID.String(), "status": string(result.Status)}, result.SiteID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(printerJSON(result))
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/printers/", func(w http.ResponseWriter, r *http.Request) {
		if !requireWritable(w, r) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/printers/")
		ensurePrinterSite := func(id uuid.UUID) bool {
			entity, err := printerService.GetByID(context.Background(), id)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if !sameSite(entity.SiteID, activeSiteID(r.Context())) {
				jsonError(w, "принтер другого склада", http.StatusForbidden)
				return false
			}
			return true
		}
		if strings.HasSuffix(path, "/cloud/verify") && r.Method == http.MethodPost {
			idString := strings.TrimSuffix(path, "/cloud/verify")
			id, err := uuid.Parse(idString)
			if err != nil {
				jsonError(w, "invalid printer id", http.StatusBadRequest)
				return
			}
			if !ensurePrinterSite(id) {
				return
			}
			var input struct {
				Code string `json:"code"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			entity, err := printerService.VerifyCloud(context.Background(), id, input.Code)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(printerJSON(entity))
			return
		}
		if strings.HasSuffix(path, "/lan") && r.Method == http.MethodPatch {
			idString := strings.TrimSuffix(path, "/lan")
			id, err := uuid.Parse(idString)
			if err != nil {
				jsonError(w, "invalid printer id", http.StatusBadRequest)
				return
			}
			if !ensurePrinterSite(id) {
				return
			}
			var input struct {
				Name            string `json:"name"`
				Model           string `json:"model"`
				LANHost         string `json:"lanHost"`
				LANSerial       string `json:"lanSerial"`
				LANAccessCode   string `json:"lanAccessCode"`
				LANEnabled      bool   `json:"lanEnabled"`
				CloudEnabled    bool   `json:"cloudEnabled"`
				CloudEmail      string `json:"cloudEmail"`
				CloudPassword   string `json:"cloudPassword"`
				CloudToken      string `json:"cloudToken"`
				CloudRegion     string `json:"cloudRegion"`
				CloudVerifyCode string `json:"cloudVerifyCode"`
				DefaultSpoolID  string `json:"defaultSpoolId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			updateInput := printerusecase.CreateInput{
				Name:            input.Name,
				Model:           input.Model,
				LANHost:         input.LANHost,
				LANSerial:       input.LANSerial,
				LANAccessCode:   input.LANAccessCode,
				LANEnabled:      input.LANEnabled,
				CloudEnabled:    input.CloudEnabled,
				CloudEmail:      input.CloudEmail,
				CloudPassword:   input.CloudPassword,
				CloudToken:      input.CloudToken,
				CloudRegion:     input.CloudRegion,
				CloudVerifyCode: input.CloudVerifyCode,
			}
			if input.DefaultSpoolID != "" {
				spoolID, err := uuid.Parse(input.DefaultSpoolID)
				if err != nil {
					jsonError(w, "invalid default spool id", http.StatusBadRequest)
					return
				}
				updateInput.DefaultSpoolID = spoolID
			}
			entity, err := printerService.UpdateLAN(context.Background(), id, updateInput)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(printerJSON(entity))
			return
		}
		if r.Method != http.MethodDelete {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := uuid.Parse(path)
		if err != nil {
			jsonError(w, "invalid printer id", http.StatusBadRequest)
			return
		}
		if !ensurePrinterSite(id) {
			return
		}
		if err := printerService.Delete(context.Background(), id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
