package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
	spoolusecase "filamenttracker/internal/usecase/spool"

	"github.com/google/uuid"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	// Isolate from local .env / .env.local (admin auth, docker hostnames).
	t.Setenv("APP_ENV", "development")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("ADMIN_PASSWORD_HASH", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("ALLOW_INMEMORY", "1")
	return NewRouter()
}

func TestRouterHealthEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	testRouter(t).ServeHTTP(w, r)

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

	testRouter(t).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRouterInventorySummaryEndpoint(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/inventory/summary", nil)
	w := httptest.NewRecorder()

	testRouter(t).ServeHTTP(w, r)

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

	testRouter(t).ServeHTTP(w, r)

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

	currentRemaining := spoolusecase.LiveRemaining(spool, jobs)
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

	atStart := spoolusecase.LiveRemaining(spool, []printjobdomain.PrintJob{{
		SpoolID:         spool.ID,
		Status:          printjobdomain.StatusPrinting,
		Progress:        0,
		EstimatedWeight: 220,
	}})
	if atStart != 1000 {
		t.Fatalf("current remaining at 0%% = %d, want 1000", atStart)
	}
	atEnd := spoolusecase.LiveRemaining(spool, []printjobdomain.PrintJob{{
		SpoolID:         spool.ID,
		Status:          printjobdomain.StatusPrinting,
		Progress:        100,
		EstimatedWeight: 220,
	}})
	if atEnd != 780 {
		t.Fatalf("current remaining at 100%% = %d, want 780", atEnd)
	}

	stale100 := spoolusecase.LiveRemaining(spool, []printjobdomain.PrintJob{{
		SpoolID:          spool.ID,
		Status:           printjobdomain.StatusPrinting,
		Progress:         100,
		RemainingMinutes: 40,
		EstimatedWeight:  220,
	}})
	if stale100 <= 780 {
		t.Fatalf("stale 100%% with ETA should still count unused filament, got %d", stale100)
	}
	if !strings.Contains(page, `href="https://example.com/"`) {
		t.Fatalf("rendered page should link home to public origin, got: %s", page)
	}
}

func TestPublicSpoolPageAndQRImage(t *testing.T) {
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "filament.up.railway.app")
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:8080")

	handler := testRouter(t)

	createBody := `{"material":"PLA","color":"чёрный","manufacturer":"Bambu Lab","initialWeight":1000,"price":30}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/spools", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRes.Code, createRes.Body.String())
	}
	var created map[string]any
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	token, _ := created["qr_token"].(string)
	if token == "" {
		t.Fatal("missing qr_token")
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/spool/"+token, nil)
	pageRes := httptest.NewRecorder()
	handler.ServeHTTP(pageRes, pageReq)
	if pageRes.Code != http.StatusOK {
		t.Fatalf("public page status = %d body=%s", pageRes.Code, pageRes.Body.String())
	}
	body := pageRes.Body.String()
	if !strings.Contains(body, "На главную") {
		t.Fatalf("public page missing home link: %s", body)
	}
	if !strings.Contains(body, `href="https://filament.up.railway.app/"`) {
		t.Fatalf("public page home link should use railway origin, got: %s", body)
	}
	if !strings.Contains(body, "Остаток на данный момент") {
		t.Fatalf("public page missing stats: %s", body)
	}

	qrReq := httptest.NewRequest(http.MethodGet, "/public/spools/qr/"+token, nil)
	qrReq.Host = "localhost:8080"
	qrRes := httptest.NewRecorder()
	handler.ServeHTTP(qrRes, qrReq)
	if qrRes.Code != http.StatusOK {
		t.Fatalf("qr status = %d body=%s", qrRes.Code, qrRes.Body.String())
	}
	if ct := qrRes.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("qr content-type = %s", ct)
	}
	if len(qrRes.Body.Bytes()) < 100 {
		t.Fatal("qr image too small")
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
