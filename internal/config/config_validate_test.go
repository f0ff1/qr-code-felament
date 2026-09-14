package config

import "testing"

func TestLoadDotEnvOverlay(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "9090")
	s := Load()
	if s.Port != "9090" {
		t.Fatalf("PORT = %q", s.Port)
	}
}

func TestValidateProductionRequiresSecrets(t *testing.T) {
	s := Settings{
		AppEnv:            EnvProduction,
		Port:              "8080",
		RequirePostgres:   true,
		DatabaseURL:       "postgres://x",
		BambuSecretsKey:   "",
		SessionSecret:     "short",
		AdminPassword:     "",
		AdminPasswordHash: "",
	}
	if err := s.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateDevelopmentOK(t *testing.T) {
	s := Settings{AppEnv: EnvDevelopment, Port: "8080", SpoolLowWeightG: 200}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
