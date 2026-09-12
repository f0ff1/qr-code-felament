package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
