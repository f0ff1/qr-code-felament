package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/domain/filament"

	"github.com/google/uuid"
)

func registerInventoryRoutes(mux *http.ServeMux, app *bootstrap.App) {
	spoolService := app.SpoolService
	inventoryService := app.InventoryService
	forecastService := app.ForecastService
	notifier := app.NotificationService

	mux.HandleFunc("/api/inventory/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		summary, err := inventoryService.GetSummary(context.Background())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload := make([]map[string]any, 0, len(summary))
		for _, item := range summary {
			payload = append(payload, map[string]any{
				"material": item.Material,
				"color":    item.Color,
				"total":    item.Total,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": payload})
	})

	mux.HandleFunc("/api/inventory/transactions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		transactions, err := inventoryService.ListTransactions(context.Background())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload := make([]map[string]any, 0, len(transactions))
		for _, tx := range transactions {
			payload = append(payload, map[string]any{
				"id":         tx.ID,
				"spool_id":   tx.SpoolID,
				"type":       tx.Type,
				"weight":     tx.Weight,
				"created_at": tx.CreatedAt.Format(time.RFC3339),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	mux.HandleFunc("/api/inventory/consume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			SpoolID string `json:"spoolId"`
			Weight  int    `json:"weight"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, "invalid payload", http.StatusBadRequest)
			return
		}
		spoolID, err := uuid.Parse(input.SpoolID)
		if err != nil {
			jsonError(w, "invalid spool id", http.StatusBadRequest)
			return
		}
		if err := inventoryService.RecordConsumption(context.Background(), spoolID, input.Weight); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "consumed"})
	})

	mux.HandleFunc("/api/forecast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		requiredWeightParam := r.URL.Query().Get("requiredWeight")
		requiredWeight, err := strconv.Atoi(requiredWeightParam)
		if err != nil || requiredWeight < 0 {
			jsonError(w, "requiredWeight must be a non-negative integer", http.StatusBadRequest)
			return
		}
		spools, err := spoolService.List(context.Background())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		availableWeight := 0
		for _, spool := range spools {
			availableWeight += spool.CurrentWeight
		}
		missing := forecastService.EstimateRequiredWeight(availableWeight, requiredWeight)
		if missing < 0 {
			missing = 0
		}
		payload := map[string]any{
			"available":          availableWeight,
			"requiredWeight":     requiredWeight,
			"missing":            missing,
			"canFinish":          forecastService.CanFinishPrint(availableWeight, requiredWeight),
			"estimatedRemaining": forecastService.EstimateRemaining(availableWeight, 300).String(),
			"explanation":        forecastService.Explain(availableWeight, requiredWeight, 300),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	mux.Handle("/events", NewSSEHandler(notifier))
	mux.HandleFunc("/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items := notifier.ListRecent(50)
		payload := make([]map[string]any, 0, len(items))
		for i := len(items) - 1; i >= 0; i-- {
			evt := items[i]
			payload = append(payload, map[string]any{
				"id":         evt.ID,
				"type":       evt.Type,
				"message":    evt.Message,
				"created_at": evt.CreatedAt.UTC().Format(time.RFC3339),
				"payload":    evt.Payload,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/api/filament-catalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"brands":    filament.Brands,
			"materials": filament.Materials,
			"colors":    filament.ColorOptions(),
		})
	})
}
