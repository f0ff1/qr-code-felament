package printer

import (
	"context"
	"strings"
	"testing"

	cloudaccount "filamenttracker/internal/domain/cloudaccount"
	"filamenttracker/internal/infrastructure/printer/mock"
	"filamenttracker/internal/infrastructure/secrets"
	"filamenttracker/internal/repository/memory"
)

func TestCloudAccountSecretsRoundTripAndLogout(t *testing.T) {
	repo := memory.NewPrinterRepository()
	accounts := memory.NewCloudAccountRepository()
	box := secrets.NewBox("unit-test-secret")
	service := NewServiceWithSecrets(repo, mock.Adapter{}, accounts, box)

	pass, err := box.Seal("secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := box.Seal("tok-123")
	if err != nil {
		t.Fatal(err)
	}
	account := cloudaccount.New("user@bambu.test", pass, tok, "us")
	if err := accounts.Upsert(context.Background(), account); err != nil {
		t.Fatal(err)
	}

	loaded, err := service.GetCloudAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Linked() || loaded.Email != "user@bambu.test" {
		t.Fatalf("expected linked public account, got %+v", loaded)
	}
	if loaded.Password != "" || loaded.Token == "tok-123" {
		t.Fatalf("secrets must not leak via GetCloudAccount: %+v", loaded)
	}

	raw, err := accounts.GetLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw.Password, "enc:v1:") || !strings.HasPrefix(raw.Token, "enc:v1:") {
		t.Fatalf("expected sealed secrets in store, got password=%q token=%q", raw.Password, raw.Token)
	}

	openedPass := box.MustOpen(raw.Password)
	openedTok := box.MustOpen(raw.Token)
	if openedPass != "secret-pass" || openedTok != "tok-123" {
		t.Fatalf("open mismatch: %q / %q", openedPass, openedTok)
	}

	if err := service.LogoutCloud(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.GetLatest(context.Background()); err == nil {
		t.Fatal("expected account removed after logout")
	}
}

func TestSaveAccountSealsSecrets(t *testing.T) {
	repo := memory.NewPrinterRepository()
	accounts := memory.NewCloudAccountRepository()
	box := secrets.NewBox("unit-test-secret-2")
	service := NewServiceWithSecrets(repo, mock.Adapter{}, accounts, box)

	if err := service.saveAccount(context.Background(), "a@b.c", "plain-pass", "plain-token", "us"); err != nil {
		t.Fatal(err)
	}
	raw, err := accounts.GetLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if raw.Password == "plain-pass" || raw.Token == "plain-token" {
		t.Fatalf("expected sealed values, got %+v", raw)
	}
	if box.MustOpen(raw.Password) != "plain-pass" || box.MustOpen(raw.Token) != "plain-token" {
		t.Fatalf("seal/open failed: %+v", raw)
	}
}
