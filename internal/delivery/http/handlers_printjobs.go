package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"filamenttracker/internal/bootstrap"
	printjobusecase "filamenttracker/internal/usecase/printjob"

	"github.com/google/uuid"
)

func registerPrintJobRoutes(mux *http.ServeMux, app *bootstrap.App) {
	printJobService := app.PrintJobService
	notifier := app.NotificationService

	mux.HandleFunc("/api/print-jobs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jobs, err := printJobService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(jobs))
			for _, job := range jobs {
				item := map[string]any{
					"id":                     job.ID.String(),
					"printer_id":             job.PrinterID.String(),
					"status":                 string(job.Status),
					"progress":               job.Progress,
					"started_at":             job.StartedAt.Format(time.RFC3339),
					"finished_at":            job.FinishedAt,
					"source":                 string(job.Source),
					"file_name":              job.FileName,
					"is_draft":               job.IsDraft,
					"external_task_id":       job.ExternalTaskID,
					"estimated_weight":       job.EstimatedWeight,
					"consumed_weight":        job.ConsumedWeight,
					"needs_filament_top_up":  printjobusecase.NeedsFilamentTopUp(job),
					"remaining_minutes":      job.RemainingMinutes,
					"estimated_duration_sec": job.EstimatedDurationSec,
					"layer_current":          job.LayerCurrent,
					"layer_total":            job.LayerTotal,
				}
				if job.ProductID != uuid.Nil {
					item["product_id"] = job.ProductID.String()
				}
				if job.SpoolID != uuid.Nil {
					item["spool_id"] = job.SpoolID.String()
				}
				payload = append(payload, item)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				PrinterID string `json:"printerId"`
				ProductID string `json:"productId"`
				SpoolID   string `json:"spoolId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			printerID, err := uuid.Parse(input.PrinterID)
			if err != nil {
				jsonError(w, "invalid printer id", http.StatusBadRequest)
				return
			}
			productID, err := uuid.Parse(input.ProductID)
			if err != nil {
				jsonError(w, "invalid product id", http.StatusBadRequest)
				return
			}
			spoolID, err := uuid.Parse(input.SpoolID)
			if err != nil {
				jsonError(w, "invalid spool id", http.StatusBadRequest)
				return
			}
			job, err := printJobService.Start(context.Background(), printerID, productID, spoolID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("print_started", "Задача печати запущена", map[string]any{"job_id": job.ID.String(), "status": string(job.Status)})
			if printjobusecase.NeedsFilamentTopUp(job) {
				notifier.Publish("filament_short", "На катушке не хватает пластика — догрузите во время печати", map[string]any{
					"job_id":           job.ID.String(),
					"estimated_weight": job.EstimatedWeight,
					"consumed_weight":  job.ConsumedWeight,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                    job.ID.String(),
				"status":                string(job.Status),
				"needs_filament_top_up": printjobusecase.NeedsFilamentTopUp(job),
				"estimated_weight":      job.EstimatedWeight,
				"consumed_weight":       job.ConsumedWeight,
			})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/print-jobs/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/print-jobs/")
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "confirm" && r.Method == http.MethodPost {
			id, err := uuid.Parse(parts[0])
			if err != nil {
				jsonError(w, "invalid job id", http.StatusBadRequest)
				return
			}
			var input struct {
				ProductID string `json:"productId"`
				SpoolID   string `json:"spoolId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			productID, err := uuid.Parse(input.ProductID)
			if err != nil {
				jsonError(w, "invalid product id", http.StatusBadRequest)
				return
			}
			spoolID, err := uuid.Parse(input.SpoolID)
			if err != nil {
				jsonError(w, "invalid spool id", http.StatusBadRequest)
				return
			}
			job, err := printJobService.ConfirmDraft(context.Background(), id, productID, spoolID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("print_started", "Черновик печати подтверждён", map[string]any{"job_id": job.ID.String()})
			if printjobusecase.NeedsFilamentTopUp(job) {
				notifier.Publish("filament_short", "На катушке не хватает пластика — догрузите во время печати", map[string]any{
					"job_id":           job.ID.String(),
					"estimated_weight": job.EstimatedWeight,
					"consumed_weight":  job.ConsumedWeight,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                    job.ID.String(),
				"status":                string(job.Status),
				"is_draft":              job.IsDraft,
				"needs_filament_top_up": printjobusecase.NeedsFilamentTopUp(job),
			})
			return
		}
		if len(parts) != 2 {
			if r.Method == http.MethodDelete {
				id, err := uuid.Parse(path)
				if err != nil {
					jsonError(w, "invalid job id", http.StatusBadRequest)
					return
				}
				if err := printJobService.Delete(context.Background(), id); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			jsonError(w, "invalid request path", http.StatusBadRequest)
			return
		}
		id, err := uuid.Parse(parts[0])
		if err != nil {
			jsonError(w, "invalid job id", http.StatusBadRequest)
			return
		}
		switch parts[1] {
		case "pause":
			if r.Method != http.MethodPost {
				jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := printJobService.Pause(context.Background(), id); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "resume":
			if r.Method != http.MethodPost {
				jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := printJobService.Resume(context.Background(), id); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "delete":
			if r.Method != http.MethodDelete {
				jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := printJobService.Delete(context.Background(), id); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			jsonError(w, "invalid action", http.StatusBadRequest)
		}
	})
}
