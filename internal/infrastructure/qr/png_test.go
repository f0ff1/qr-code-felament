package qr

import (
	"bytes"
	"testing"
)

func TestPNG(t *testing.T) {
	data, err := PNG("demo-token")
	if err != nil {
		t.Fatalf("PNG() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("PNG() returned empty bytes")
	}
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		t.Fatal("PNG() did not return PNG data")
	}
}
