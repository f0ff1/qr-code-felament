package qr

import "github.com/skip2/go-qrcode"

// PNG encodes content as a QR code PNG (256px, medium ECC).
func PNG(content string) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, 256)
}
