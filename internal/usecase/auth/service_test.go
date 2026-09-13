package auth_test

import (
	"testing"
	"time"

	"filamenttracker/internal/config"
	"filamenttracker/internal/infrastructure/session"
	authusecase "filamenttracker/internal/usecase/auth"
)

func TestLoginLogoutRoundTrip(t *testing.T) {
	cfg := config.Settings{
		AppEnv:        config.EnvDevelopment,
		AdminUsername: "admin",
		AdminPassword: "secret",
		SessionTTL:    time.Hour,
	}
	svc, err := authusecase.NewService(cfg, session.NewStore(cfg.SessionTTL))
	if err != nil {
		t.Fatal(err)
	}
	if !svc.Enabled() {
		t.Fatal("expected auth enabled")
	}
	sess, err := svc.Login("admin", "secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authenticate(sess.ID)
	if err != nil || got.Username != "admin" {
		t.Fatalf("authenticate: %+v %v", got, err)
	}
	svc.Logout(sess.ID)
	if _, err := svc.Authenticate(sess.ID); err == nil {
		t.Fatal("expected unauthorized after logout")
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	cfg := config.Settings{
		AppEnv:        config.EnvDevelopment,
		AdminUsername: "admin",
		AdminPassword: "secret",
		SessionTTL:    time.Hour,
	}
	svc, err := authusecase.NewService(cfg, session.NewStore(cfg.SessionTTL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login("admin", "wrong"); err == nil {
		t.Fatal("expected error")
	}
}
