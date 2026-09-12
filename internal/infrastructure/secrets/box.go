package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const prefix = "enc:v1:"

// Box seals secrets at rest with AES-GCM.
type Box struct {
	key []byte
}

// NewBoxFromEnv builds a box from BAMBU_SECRETS_KEY or APP_SECRET.
// If neither is set, derives a key from DATABASE_URL (stable per deploy) or a warning fallback.
func NewBoxFromEnv() *Box {
	raw := strings.TrimSpace(os.Getenv("BAMBU_SECRETS_KEY"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("APP_SECRET"))
	}
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if raw == "" {
		raw = "filament-tracker-dev-secret-change-me"
	}
	sum := sha256.Sum256([]byte(raw))
	return &Box{key: sum[:]}
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
