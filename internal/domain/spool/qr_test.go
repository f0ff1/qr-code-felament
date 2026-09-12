package spool

import (
	"bytes"
	"testing"
)

func TestGenerateQRPNG(t *testing.T) {
	data, err := GenerateQRPNG("demo-token")
	if err != nil {
		t.Fatalf("GenerateQRPNG() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GenerateQRPNG() returned empty bytes")
	}
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		t.Fatal("GenerateQRPNG() did not return PNG data")
	}
}
