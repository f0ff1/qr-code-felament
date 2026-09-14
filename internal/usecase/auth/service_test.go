package auth

import (
	"context"
	"testing"

	"filamenttracker/internal/config"
	orgdomain "filamenttracker/internal/domain/org"
	userdomain "filamenttracker/internal/domain/user"
	"filamenttracker/internal/infrastructure/session"
	"filamenttracker/internal/repository/memory"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginAndAuthenticate(t *testing.T) {
	users := memory.NewUserRepository()
	resets := memory.NewPasswordResetRepository()
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	u := userdomain.User{
		ID:             uuid.New(),
		OrganizationID: orgdomain.DefaultOrganizationID,
		Username:       "admin",
		PasswordHash:   string(hash),
		Role:           userdomain.RoleAdmin,
		IsActive:       true,
		SiteIDs:        []uuid.UUID{orgdomain.DefaultSiteID},
	}
	if err := users.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if err := users.SetSites(context.Background(), u.ID, u.SiteIDs); err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(config.Settings{SessionTTL: 0, AppEnv: config.EnvTest}, session.NewStore(0), users, resets, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.Login(context.Background(), "admin", "admin123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	got, err := svc.Authenticate(sess.ID)
	if err != nil || got.Username != "admin" || got.Role != "admin" {
		t.Fatalf("authenticate = %#v err=%v", got, err)
	}
}

func TestBootstrapAdmin(t *testing.T) {
	users := memory.NewUserRepository()
	svc, err := NewService(config.Settings{
		AppEnv:        config.EnvDevelopment,
		AdminUsername: "boss",
		AdminPassword: "secret12",
	}, session.NewStore(0), users, memory.NewPasswordResetRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.BootstrapAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	n, _ := users.Count(context.Background())
	if n != 1 {
		t.Fatalf("count=%d", n)
	}
	_, err = svc.Login(context.Background(), "boss", "secret12")
	if err != nil {
		t.Fatal(err)
	}
}
