package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/domain/filament"
	spooldomain "filamenttracker/internal/domain/spool"
	spoolusecase "filamenttracker/internal/usecase/spool"

	"github.com/google/uuid"
)

func registerSpoolRoutes(mux *http.ServeMux, app *bootstrap.App) {
	spoolService := app.SpoolService
	printJobService := app.PrintJobService
	notifier := app.NotificationService

	mux.HandleFunc("/api/spools", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			spools, err := spoolService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(spools))
			jobs, jobsErr := printJobService.List(context.Background())
			if jobsErr != nil {
				jobs = nil
			}
			for _, spool := range spools {
				future := spool.CurrentWeight
				current := future
				if jobsErr == nil {
					current = spoolusecase.LiveRemaining(spool, jobs)
				}
				payload = append(payload, map[string]any{
					"id":                spool.ID.String(),
					"material":          string(spool.Material),
					"color":             spool.Color,
					"manufacturer":      spool.Manufacturer,
					"initial_weight":    spool.InitialWeight,
					"remaining_weight":  future,
					"current_remaining": current,
					"price":             spool.Price,
					"status":            string(spool.Status),
					"qr_token":          spool.QRToken,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				Material      string  `json:"material"`
				Color         string  `json:"color"`
				Manufacturer  string  `json:"manufacturer"`
				InitialWeight int     `json:"initialWeight"`
				Price         float64 `json:"price"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if !filament.IsKnownMaterial(input.Material) {
				jsonError(w, "некорректный тип пластика — выберите значение из списка Bambu", http.StatusBadRequest)
				return
			}
			if !filament.IsKnownBrand(input.Manufacturer) {
				jsonError(w, "некорректный производитель — выберите значение из списка Bambu", http.StatusBadRequest)
				return
			}
			if !filament.IsKnownColor(input.Color) {
				jsonError(w, "некорректный цвет — выберите цвет из таблицы матчинга", http.StatusBadRequest)
				return
			}
			if input.InitialWeight <= 0 || input.Price < 0 {
				jsonError(w, "некорректный вес или цена", http.StatusBadRequest)
				return
			}
			entity, err := spoolService.Create(context.Background(), spooldomain.Material(input.Material), input.Color, input.Manufacturer, input.InitialWeight, input.Price)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("spool_created", "New spool created", map[string]any{"spool_id": entity.ID.String(), "status": string(entity.Status)})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": entity.ID.String(), "qr_token": entity.QRToken})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/spools/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/spools/")
		if strings.HasSuffix(path, "/weight") {
			if r.Method != http.MethodPatch {
				jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			idString := strings.TrimSuffix(path, "/weight")
			id, err := uuid.Parse(idString)
			if err != nil {
				jsonError(w, "invalid spool id", http.StatusBadRequest)
				return
			}
			var input struct {
				RemainingWeight int `json:"remainingWeight"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if err := spoolService.UpdateRemaining(context.Background(), id, input.RemainingWeight); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodDelete {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, err := uuid.Parse(path)
		if err != nil {
			jsonError(w, "invalid spool id", http.StatusBadRequest)
			return
		}
		if err := spoolService.Delete(context.Background(), id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
