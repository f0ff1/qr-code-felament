package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

func TestRouterHealthEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("body = %s, want ok", w.Body.String())
	}
}

func TestRouterSpoolListEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/spools", nil)
	w := httptest.NewRecorder()

	NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRouterInventorySummaryEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/inventory/summary", nil)
	w := httptest.NewRecorder()

	NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "material") && !strings.Contains(w.Body.String(), "[]") {
		t.Fatalf("body = %s, want inventory summary payload", w.Body.String())
	}
}

func TestRouterForecastEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/forecast?requiredWeight=200", nil)
	w := httptest.NewRecorder()

	NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "available") && !strings.Contains(w.Body.String(), "requiredWeight") {
		t.Fatalf("body = %s, want forecast payload", w.Body.String())
	}
}

func TestRenderSpoolPublicPageUsesCurrentRemaining(t *testing.T) {
	spool := spooldomain.Spool{
		ID:            mustUUID(t, "11111111-1111-4111-8111-111111111111"),
		Material:      spooldomain.MaterialPLA,
		Color:         "Черный",
		Manufacturer:  "Bambu",
		InitialWeight: 1000,
		CurrentWeight: 780,
		QRToken:       "abc12345",
		Status:        spooldomain.StatusAvailable,
	}

	jobs := []printjobdomain.PrintJob{{
		SpoolID:         spool.ID,
		Status:          printjobdomain.StatusPrinting,
		Progress:        50,
		EstimatedWeight: 220,
	}}

	currentRemaining := spoolCurrentDisplayRemaining(spool, jobs)
	page := renderSpoolPublicPage(spool, currentRemaining, spool.CurrentWeight, "https://example.com")

	if !strings.Contains(page, "Остаток на данный момент") {
		t.Fatalf("rendered page missing current-remaining label: %s", page)
	}
	if !strings.Contains(page, "Остаток в будущем") {
		t.Fatalf("rendered page missing future-remaining label: %s", page)
	}
	if !strings.Contains(page, "890 г") {
		t.Fatalf("rendered page should show current remaining with active print progress, got: %s", page)
	}
	if !strings.Contains(page, "780 г") {
		t.Fatalf("rendered page should show future remaining from DB, got: %s", page)
	}
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
