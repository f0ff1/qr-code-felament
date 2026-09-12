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
	StatusDraft     Status = "draft"
)

type Source string

const (
	SourceManual Source = "manual"
	SourceBambu  Source = "bambu"
)

type PrintJob struct {
	ID              uuid.UUID
	PrinterID       uuid.UUID
	ProductID       uuid.UUID
	SpoolID         uuid.UUID
	Status          Status
	Source          Source
	ExternalTaskID  string
	FileName        string
	IsDraft         bool
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
		Source:          SourceManual,
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
	j.IsDraft = false
}

func (j *PrintJob) Fail() {
	j.Status = StatusFailed
	finishedAt := time.Now()
	j.FinishedAt = &finishedAt
	j.UpdatedAt = time.Now()
}

func (j *PrintJob) Cancel() {
	j.Status = StatusCancelled
	finishedAt := time.Now()
	j.FinishedAt = &finishedAt
	j.UpdatedAt = time.Now()
}

func IsActive(status Status) bool {
	return status == StatusQueued || status == StatusPrinting || status == StatusPaused || status == StatusDraft
}
