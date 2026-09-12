package printjob

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusPrinting  Status = "printing"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type PrintJob struct {
	ID              uuid.UUID
	PrinterID       uuid.UUID
	ProductID       uuid.UUID
	SpoolID         uuid.UUID
	Status          Status
	Progress        float64
	StartedAt       time.Time
	FinishedAt      *time.Time
	EstimatedWeight int
	ConsumedWeight  int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewPrintJob(printerID, productID, spoolID uuid.UUID, estimatedWeight int) PrintJob {
	now := time.Now()
	return PrintJob{
		ID:              uuid.New(),
		PrinterID:       printerID,
		ProductID:       productID,
		SpoolID:         spoolID,
		Status:          StatusQueued,
		Progress:        0,
		StartedAt:       now,
		EstimatedWeight: estimatedWeight,
		ConsumedWeight:  0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (j *PrintJob) Start() {
	j.Status = StatusPrinting
	j.UpdatedAt = time.Now()
}

func (j *PrintJob) Pause() {
	j.Status = StatusPaused
	j.UpdatedAt = time.Now()
}

func (j *PrintJob) Resume() {
	j.Status = StatusPrinting
	j.UpdatedAt = time.Now()
}

func (j *PrintJob) Complete() {
	j.Status = StatusCompleted
	j.Progress = 100
	finishedAt := time.Now()
	j.FinishedAt = &finishedAt
	j.UpdatedAt = time.Now()
}
