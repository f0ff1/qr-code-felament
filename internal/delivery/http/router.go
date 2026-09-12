package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/config"
	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/infrastructure/printer/mock"
	"filamenttracker/internal/repository/memory"
	postgresrepo "filamenttracker/internal/repository/postgres"
	inventoryusecase "filamenttracker/internal/usecase/inventory"
	notificationusecase "filamenttracker/internal/usecase/notification"
	predictionusecase "filamenttracker/internal/usecase/prediction"
	printerusecase "filamenttracker/internal/usecase/printer"
	printjobusecase "filamenttracker/internal/usecase/printjob"
	productusecase "filamenttracker/internal/usecase/product"
	runtimeusecase "filamenttracker/internal/usecase/runtime"
	spoolusecase "filamenttracker/internal/usecase/spool"

	"github.com/google/uuid"
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

func spoolCurrentDisplayRemaining(spoolEntity spooldomain.Spool, jobs []printjobdomain.PrintJob) int {
	currentRemaining := spoolEntity.CurrentWeight
	consumedFromJobs := 0
	for _, job := range jobs {
		if job.SpoolID != spoolEntity.ID || !isActivePrintJobStatus(job.Status) {
			continue
		}
		progress := max(0.0, job.Progress)
		if progress > 100 {
			progress = 100
		}
		consumedFromJobs += int(float64(job.EstimatedWeight) * (progress / 100.0))
	}
	return max(0, currentRemaining+consumedFromJobs)
}

func isActivePrintJobStatus(status printjobdomain.Status) bool {
	return status == printjobdomain.StatusQueued || status == printjobdomain.StatusPrinting || status == printjobdomain.StatusPaused
}

func renderSpoolPublicPage(spoolEntity spooldomain.Spool, currentRemaining, projectedRemaining int, baseURL string) string {
	statusLabel := "достаточно"
	switch {
	case spoolEntity.Status == spooldomain.StatusInUse:
		statusLabel = "в работе"
	case currentRemaining <= 0:
		statusLabel = "закончился"
	case currentRemaining < 200:
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

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "error",
		"code":    status,
		"message": message,
	})
}

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	runtime := bootstrap.NewRuntime()

	var (
		spoolRepo     spooldomain.Repository
		printerRepo   printerusecase.Repository
		productRepo   productusecase.ProductRepository
		printJobRepo  printjobusecase.PrintJobRepository
		inventoryRepo inventoryusecase.InventoryRepository
	)

	if runtime.DB != nil {
		spoolRepo = postgresrepo.NewSpoolRepository(runtime.DB)
		printerRepo = postgresrepo.NewPrinterRepository(runtime.DB)
		productRepo = postgresrepo.NewProductRepository(runtime.DB)
		printJobRepo = postgresrepo.NewPrintJobRepository(runtime.DB)
		inventoryRepo = postgresrepo.NewInventoryRepository(runtime.DB)
		log.Println("using postgres repositories")
	} else {
		spoolRepo = memory.NewRepository()
		printerRepo = memory.NewPrinterRepository()
		productRepo = memory.NewProductRepository()
		printJobRepo = memory.NewPrintJobRepository()
		inventoryRepo = memory.NewInventoryRepository()
		log.Println("using in-memory repositories")
	}

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := productusecase.NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)
	inventoryService := inventoryusecase.NewService(spoolRepo, inventoryRepo)
	forecastService := predictionusecase.NewEstimator()
	runtimeSynchronizer := runtimeusecase.NewSynchronizer(spoolRepo, printerRepo, printJobRepo, productRepo, runtime.Redis)
	syncRuntime := func(ctx context.Context) {
		if err := runtimeSynchronizer.Sync(ctx); err != nil {
			log.Printf("sync runtime state: %v", err)
		}
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			syncRuntime(ctx)
			cancel()
		}
	}()
	notifier := notificationusecase.NewService()
	demoCtx, demoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	ensureDemoData(demoCtx, spoolService, printerService, productService, printJobService)
	demoCancel()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "memory"
		if runtime.DB != nil {
			dbStatus = "postgres"
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := runtime.DB.PingContext(ctx); err != nil {
				jsonError(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "db": dbStatus})
	})

	mux.HandleFunc("/api/spools", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			spools, err := spoolService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(spools))
			for _, spool := range spools {
				payload = append(payload, map[string]any{
					"id":               spool.ID.String(),
					"material":         string(spool.Material),
					"color":            spool.Color,
					"manufacturer":     spool.Manufacturer,
					"initial_weight":   spool.InitialWeight,
					"remaining_weight": spool.CurrentWeight,
					"price":            spool.Price,
					"status":           string(spool.Status),
					"qr_token":         spool.QRToken,
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

	mux.HandleFunc("/api/printers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			printers, err := printerService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(printers))
			for _, printer := range printers {
				payload = append(payload, map[string]any{
					"id":     printer.ID.String(),
					"name":   printer.Name,
					"model":  printer.Model,
					"status": string(printer.Status),
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			entity, err := printerService.Create(context.Background(), input.Name, input.Model)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("printer_created", "Printer registered", map[string]any{"printer_id": entity.ID.String(), "status": string(entity.Status)})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": entity.ID.String(), "status": string(entity.Status)})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/printers/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idString := strings.TrimPrefix(r.URL.Path, "/api/printers/")
		id, err := uuid.Parse(idString)
		if err != nil {
			jsonError(w, "invalid printer id", http.StatusBadRequest)
			return
		}
		if err := printerService.Delete(context.Background(), id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			products, err := productService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			payload := make([]map[string]any, 0, len(products))
			for _, product := range products {
				payload = append(payload, map[string]any{
					"id":                   product.ID.String(),
					"name":                 product.Name,
					"description":          product.Description,
					"material":             product.Material,
					"estimated_weight":     product.EstimatedWeight,
					"estimated_print_time": product.EstimatedPrintTime.String(),
					"price":                product.Price,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			var input struct {
				Name               string  `json:"name"`
				Description        string  `json:"description"`
				Material           string  `json:"material"`
				EstimatedWeight    int     `json:"estimatedWeight"`
				EstimatedPrintTime string  `json:"estimatedPrintTime"`
				Price              float64 `json:"price"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, "invalid payload", http.StatusBadRequest)
				return
			}
			duration, err := timeFromString(input.EstimatedPrintTime)
			if err != nil {
				jsonError(w, "invalid duration", http.StatusBadRequest)
				return
			}
			entity, err := productService.Create(context.Background(), input.Name, input.Description, input.Material, input.EstimatedWeight, duration, input.Price)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("product_created", "Product created", map[string]any{"product_id": entity.ID.String(), "material": entity.Material})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": entity.ID.String(), "name": entity.Name})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/products/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idString := strings.TrimPrefix(r.URL.Path, "/api/products/")
		id, err := uuid.Parse(idString)
		if err != nil {
			jsonError(w, "invalid product id", http.StatusBadRequest)
			return
		}
		if err := productService.Delete(context.Background(), id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

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
				payload = append(payload, map[string]any{
					"id":          job.ID.String(),
					"printer_id":  job.PrinterID.String(),
					"product_id":  job.ProductID.String(),
					"spool_id":    job.SpoolID.String(),
					"status":      string(job.Status),
					"progress":    job.Progress,
					"started_at":  job.StartedAt.Format(time.RFC3339),
					"finished_at": job.FinishedAt,
				})
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
			notifier.Publish("print_started", "Print job started", map[string]any{"job_id": job.ID.String(), "status": string(job.Status)})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": job.ID.String(), "status": string(job.Status)})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/print-jobs/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/print-jobs/")
		parts := strings.Split(path, "/")
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
		png, err := spooldomain.GenerateQRPNG(qrTarget)
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
		currentRemaining := spoolCurrentDisplayRemaining(spoolEntity, jobs)
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

	mux.Handle("/events", NewSSEHandler(notifier))

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
	return mux
}

func ensureDemoData(ctx context.Context, spoolService *spoolusecase.Service, printerService *printerusecase.Service, productService *productusecase.Service, printJobService *printjobusecase.Service) {
	spools, err := spoolService.List(ctx)
	if err != nil || len(spools) > 0 {
		return
	}

	printerA, err := printerService.Create(ctx, "Prusa MK4", "MK4")
	if err == nil {
		_ = printerA
	}
	printerB, err := printerService.Create(ctx, "Bambu X1", "X1")
	if err == nil {
		_ = printerB
	}

	spoolA, err := spoolService.Create(ctx, spooldomain.MaterialPLA, "Черный", "Bambu", 1000, 35)
	if err != nil {
		return
	}
	spoolB, err := spoolService.Create(ctx, spooldomain.MaterialPETG, "Белый", "eSUN", 800, 28)
	if err != nil {
		return
	}

	productA, err := productService.Create(ctx, "Кронштейн A", "Кронштейн для корпуса", "PLA", 220, 2*time.Hour+30*time.Minute, 19.9)
	if err != nil {
		return
	}
	productB, err := productService.Create(ctx, "Корпус B", "Плоский корпус для сборки", "PETG", 310, 3*time.Hour+15*time.Minute, 24.5)
	if err != nil {
		return
	}

	_, _ = printJobService.Start(ctx, printerA.ID, productA.ID, spoolA.ID)
	_, _ = printJobService.Start(ctx, printerB.ID, productB.ID, spoolB.ID)
}

func timeFromString(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	return time.ParseDuration(raw)
}
