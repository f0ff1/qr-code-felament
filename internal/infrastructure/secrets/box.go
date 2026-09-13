package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"filamenttracker/internal/config"
)

const prefix = "enc:v1:"

// Box seals secrets at rest with AES-GCM.
type Box struct {
	key []byte
}

// NewBoxFromSettings builds a box from configured key material.
// In production Validate() must ensure a strong key exists.
// In development a dedicated weak-dev key is used only when unset (never DATABASE_URL).
func NewBoxFromSettings(cfg config.Settings) (*Box, error) {
	raw := strings.TrimSpace(cfg.BambuSecretsKey)
	if raw == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("BAMBU_SECRETS_KEY is required")
		}
		raw = "filament-tracker-local-dev-only-not-for-prod"
	}
	return NewBox(raw), nil
}

// NewBoxFromEnv loads config and builds a box (legacy helper for call sites).
func NewBoxFromEnv() *Box {
	cfg := config.Load()
	box, err := NewBoxFromSettings(cfg)
	if err != nil {
		panic(err)
	}
	return box
}

func NewBox(keyMaterial string) *Box {
	sum := sha256.Sum256([]byte(strings.TrimSpace(keyMaterial)))
	return &Box{key: sum[:]}
}

func (b *Box) Seal(plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, prefix) {
		return plaintext, nil
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.RawStdEncoding.EncodeToString(out), nil
}

func (b *Box) Open(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		// Legacy plaintext row — still usable, re-sealed on next save.
		return value, nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("decode sealed secret: %w", err)
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("sealed secret too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("open sealed secret: %w", err)
	}
	return string(plain), nil
}

func (b *Box) MustOpen(value string) string {
	out, err := b.Open(value)
	if err != nil {
		return ""
	}
	return out
}
