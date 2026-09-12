package worker

import (
	"context"
	"testing"
	"time"

	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/repository/memory"
	notificationusecase "filamenttracker/internal/usecase/notification"

	"github.com/google/uuid"
)

func TestSchedulerScanPublishesLowSpoolNotification(t *testing.T) {
	spoolRepo := memory.NewRepository()
	notifier := notificationusecase.NewService()

	spoolEntity := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "eSUN", 100, 30)
	spoolEntity.CurrentWeight = 10
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, nil, notifier)
	scheduler.Scan(context.Background())

	events := notifier.List()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "spool_low" {
		t.Fatalf("event type = %s, want spool_low", events[0].Type)
	}
}

func TestSchedulerScanPublishesPrintCompletedNotification(t *testing.T) {
	printJobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), uuid.New(), 250)
	job.Start()
	job.Complete()
	if err := printJobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create print job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(nil, printJobRepo, notifier)
	scheduler.Scan(context.Background())

	events := notifier.List()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "print_completed" {
		t.Fatalf("event type = %s, want print_completed", events[0].Type)
	}
	if events[0].Payload["job_id"] == nil {
		t.Fatal("print_completed payload missing job_id")
	}
}

func TestSchedulerRunUsesTicker(t *testing.T) {
	scheduler := NewSchedulerWithDeps(nil, nil, notificationusecase.NewService())
	scheduler.tick = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
