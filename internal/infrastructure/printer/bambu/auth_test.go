package bambu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudLoginRequestsEmailCodeOnVerifyType(t *testing.T) {
	var loginHits, codeHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/user-service/user/login"):
			loginHits++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accessToken": "",
				"loginType":   "verifyCode",
			})
		case strings.HasSuffix(r.URL.Path, "/user-service/user/sendemail/code"):
			codeHits++
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["email"] != "user@test.com" || body["type"] != "codeLogin" {
				t.Fatalf("unexpected sendemail body: %+v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cloudAPIBaseOverride = server.URL
	t.Cleanup(func() { cloudAPIBaseOverride = "" })

	result, err := CloudLogin("user@test.com", "secret", "us")
	if err != nil {
		t.Fatalf("CloudLogin: %v", err)
	}
	if !result.NeedsVerify {
		t.Fatalf("expected NeedsVerify, got %+v", result)
	}
	if loginHits != 1 || codeHits != 1 {
		t.Fatalf("loginHits=%d codeHits=%d, want 1/1", loginHits, codeHits)
	}
}

func TestCloudVerifyReturnsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "jwt-token",
			"loginType":   "",
		})
	}))
	defer server.Close()
	cloudAPIBaseOverride = server.URL
	t.Cleanup(func() { cloudAPIBaseOverride = "" })

	token, err := CloudVerify("user@test.com", "123456", "us")
	if err != nil {
		t.Fatal(err)
	}
	if token != "jwt-token" {
		t.Fatalf("token=%q", token)
	}
}

func TestAuthErrorMessagePrefersMessageField(t *testing.T) {
	got := authErrorMessage(authLoginResponse{Message: "bad password"}, `{"error":"x"}`)
	if got != "bad password" {
		t.Fatalf("got %q", got)
	}
}
