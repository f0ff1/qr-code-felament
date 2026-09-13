package spool

import (
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

type Material string

const (
	MaterialPLA  Material = "PLA"
	MaterialPETG Material = "PETG"
	MaterialABS  Material = "ABS"
)

type Status string

const (
	StatusAvailable Status = "available"
	StatusLow       Status = "low"
	StatusInUse     Status = "in_use"
	StatusEmpty     Status = "empty"
)

type Spool struct {
	ID            uuid.UUID
	QRToken       string
	Material      Material
	Color         string
	Manufacturer  string
	InitialWeight int
	CurrentWeight int
	Price         float64
	Status        Status
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewSpool(material Material, color, manufacturer string, initialWeight int, price float64) Spool {
	return Spool{
		ID:            uuid.New(),
		QRToken:       GenerateQRToken(),
		Material:      material,
		Color:         color,
		Manufacturer:  manufacturer,
		InitialWeight: initialWeight,
		CurrentWeight: initialWeight,
		Price:         price,
		Status:        StatusAvailable,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func GenerateQRToken() string {
	return uuid.NewString()[:8]
}

// LowWeightGrams is the threshold for StatusLow. Set once from config at process start.
var LowWeightGrams = 200

func GenerateQRPNG(token string) ([]byte, error) {
	return qrcode.Encode(token, qrcode.Medium, 256)
}

func (s *Spool) EffectiveStatus(inUse bool) Status {
	if inUse {
		return StatusInUse
	}
	if s.CurrentWeight <= 0 {
		return StatusEmpty
	}
	threshold := LowWeightGrams
	if threshold < 0 {
		threshold = 200
	}
	if s.CurrentWeight < threshold {
		return StatusLow
	}
	return StatusAvailable
}

func (s *Spool) Consume(weight int) {
	if weight > 0 && s.CurrentWeight >= weight {
		s.CurrentWeight -= weight
		if s.CurrentWeight <= 0 {
			s.CurrentWeight = 0
			s.Status = StatusEmpty
		} else {
			s.Status = StatusInUse
		}
		s.UpdatedAt = time.Now()
	}
}

func (s *Spool) Refill(weight int) {
	if weight > 0 {
		s.CurrentWeight += weight
		s.Status = StatusAvailable
		s.UpdatedAt = time.Now()
	}
}
