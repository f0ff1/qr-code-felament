package printer

import (
	"time"

	"github.com/google/uuid"
)

type PrinterStatus string

const (
	StatusIdle     PrinterStatus = "idle"
	StatusPrinting PrinterStatus = "printing"
	StatusPaused   PrinterStatus = "paused"
	StatusOffline  PrinterStatus = "offline"
)

type Printer struct {
	ID        uuid.UUID
	Name      string
	Model     string
	Status    PrinterStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewPrinter(name, model string) Printer {
	return Printer{
		ID:        uuid.New(),
		Name:      name,
		Model:     model,
		Status:    StatusIdle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
