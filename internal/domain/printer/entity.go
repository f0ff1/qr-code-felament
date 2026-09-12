package printer

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type PrinterStatus string

const (
	StatusIdle     PrinterStatus = "idle"
	StatusPrinting PrinterStatus = "printing"
	StatusPaused   PrinterStatus = "paused"
	StatusOffline  PrinterStatus = "offline"
	StatusError    PrinterStatus = "error"
)

type ConnectionMode string

const (
	ConnectionNone ConnectionMode = ""
	ConnectionLAN  ConnectionMode = "lan"
	ConnectionCloud ConnectionMode = "cloud"
)

type Printer struct {
	ID             uuid.UUID
	Name           string
	Model          string
	Status         PrinterStatus
	LANHost        string
	LANSerial      string
	LANAccessCode  string
	LANEnabled     bool
	CloudEnabled   bool
	CloudEmail     string
	CloudPassword  string
	CloudToken     string
	CloudRegion    string
	DefaultSpoolID uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewPrinter(name, model string) Printer {
	return Printer{
		ID:          uuid.New(),
		Name:        name,
		Model:       model,
		Status:      StatusIdle,
		CloudRegion: "us",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (p Printer) HasLANConfig() bool {
	return p.LANEnabled && !p.CloudEnabled &&
		strings.TrimSpace(p.LANHost) != "" &&
		strings.TrimSpace(p.LANSerial) != "" &&
		strings.TrimSpace(p.LANAccessCode) != ""
}

func (p Printer) HasCloudConfig() bool {
	if !p.CloudEnabled || strings.TrimSpace(p.LANSerial) == "" {
		return false
	}
	if strings.TrimSpace(p.CloudToken) != "" {
		return true
	}
	return strings.TrimSpace(p.CloudEmail) != "" && strings.TrimSpace(p.CloudPassword) != ""
}

func (p Printer) CloudLinked() bool {
	return p.CloudEnabled && strings.TrimSpace(p.CloudToken) != ""
}

func (p Printer) ConnectionMode() ConnectionMode {
	if p.CloudEnabled {
		return ConnectionCloud
	}
	if p.LANEnabled {
		return ConnectionLAN
	}
	return ConnectionNone
}
