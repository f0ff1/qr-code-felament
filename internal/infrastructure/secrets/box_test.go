package secrets

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	box := NewBox("test-secret-key")
	sealed, err := box.Seal("my-token-value")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "my-token-value" || sealed == "" {
		t.Fatalf("expected sealed value, got %q", sealed)
	}
	plain, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "my-token-value" {
		t.Fatalf("got %q", plain)
	}
}

func TestOpenLegacyPlaintext(t *testing.T) {
	box := NewBox("test-secret-key")
	plain, err := box.Open("legacy-password")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "legacy-password" {
		t.Fatalf("got %q", plain)
	}
}
