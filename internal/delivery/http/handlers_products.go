package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"filamenttracker/internal/bootstrap"
	productdomain "filamenttracker/internal/domain/product"

	"github.com/google/uuid"
)

func registerProductRoutes(mux *http.ServeMux, app *bootstrap.App) {
	productService := app.ProductService
	notifier := app.NotificationService

	mux.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			products, err := productService.List(context.Background())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			siteID := activeSiteID(r.Context())
			payload := make([]map[string]any, 0, len(products))
			for _, product := range products {
				if !sameSite(product.SiteID, siteID) {
					continue
				}
				payload = append(payload, map[string]any{
					"id":                   product.ID.String(),
					"name":                 product.Name,
					"description":          product.Description,
					"material":             product.Material,
					"estimated_weight":     product.EstimatedWeight,
					"estimated_print_time": product.EstimatedPrintTime.String(),
					"price":                product.Price,
					"price_legal":          product.PriceLegal,
					"billing_mode":         string(product.BillingMode),
					"site_id":              product.SiteID.String(),
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case http.MethodPost:
			if !requireWritable(w, r) {
				return
			}
			var input struct {
				Name               string  `json:"name"`
				Description        string  `json:"description"`
				Material           string  `json:"material"`
				EstimatedWeight    int     `json:"estimatedWeight"`
				EstimatedPrintTime string  `json:"estimatedPrintTime"`
				Price              float64 `json:"price"`
				PriceLegal         float64 `json:"priceLegal"`
				BillingMode        string  `json:"billingMode"`
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
			siteID := activeSiteID(r.Context())
			entity, err := productService.CreateForSite(context.Background(), siteID, input.Name, input.Description, input.Material, input.EstimatedWeight, duration, input.Price, input.PriceLegal, productdomain.BillingMode(input.BillingMode))
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			notifier.Publish("product_created", "Product created", withSitePayload(map[string]any{"product_id": entity.ID.String(), "material": entity.Material}, entity.SiteID))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": entity.ID.String(), "name": entity.Name})
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/products/", func(w http.ResponseWriter, r *http.Request) {
		if !requireWritable(w, r) {
			return
		}
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
		product, err := productService.GetByID(context.Background(), id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !sameSite(product.SiteID, activeSiteID(r.Context())) {
			jsonError(w, "продукт другого склада", http.StatusForbidden)
			return
		}
		if err := productService.Delete(context.Background(), id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func timeFromString(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	return time.ParseDuration(raw)
}
