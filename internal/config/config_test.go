package config

import "testing"

func TestPublicOriginPrefersRailwayDomainOverLocalhost(t *testing.T) {
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "filament-tracker-production.up.railway.app")

	settings := Settings{PublicBaseURL: "http://localhost:8080"}
	got := settings.PublicOrigin("localhost:8080", "https", false)
	want := "https://filament-tracker-production.up.railway.app"
	if got != want {
		t.Fatalf("PublicOrigin() = %q, want %q", got, want)
	}
}

func TestPublicOriginUsesExplicitPublicURL(t *testing.T) {
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "")

	settings := Settings{PublicBaseURL: "https://filament.example.com/"}
	got := settings.PublicOrigin("ignored.example", "http", false)
	if got != "https://filament.example.com" {
		t.Fatalf("PublicOrigin() = %q", got)
	}
}

func TestPublicOriginFallsBackToForwardedHost(t *testing.T) {
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "")

	settings := Settings{}
	got := settings.PublicOrigin("app.up.railway.app", "https", false)
	if got != "https://app.up.railway.app" {
		t.Fatalf("PublicOrigin() = %q", got)
	}
}
