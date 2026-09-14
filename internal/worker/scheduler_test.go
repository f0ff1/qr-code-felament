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
	spoolEntity.Status = spooldomain.StatusAvailable
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, nil, notifier, 0)
	scheduler.Scan(context.Background())

	events := notifier.List()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "spool_low" {
		t.Fatalf("event type = %s, want spool_low", events[0].Type)
	}

	// Second scan must not repeat the same low warning.
	scheduler.Scan(context.Background())
	if got := len(notifier.List()); got != 1 {
		t.Fatalf("after second scan len(events) = %d, want 1", got)
	}
}

func TestSchedulerScanDoesNotSpamCompletedPrints(t *testing.T) {
	printJobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), uuid.New(), 250)
	job.Start()
	job.Complete()
	if err := printJobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create print job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(nil, printJobRepo, notifier, 0)
	scheduler.Scan(context.Background())

	if got := len(notifier.List()); got != 0 {
		t.Fatalf("completed jobs must not be re-notified by scanner, got %d", got)
	}
}

func TestSchedulerSyncRuntimeStateRefreshesActiveStatus(t *testing.T) {
	spoolRepo := memory.NewRepository()
	jobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	spoolEntity := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "eSUN", 500, 30)
	spoolEntity.CurrentWeight = 300
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), spoolEntity.ID, 200)
	job.Status = printjobdomain.StatusPrinting
	job.Progress = 35
	job.StartedAt = time.Now().Add(-2 * time.Minute)
	if err := jobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create print job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, jobRepo, notifier, 0)
	scheduler.syncRuntimeState(context.Background())

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID spool error = %v", err)
	}
	if updatedSpool.Status != spooldomain.StatusInUse {
		t.Fatalf("updatedSpool.Status = %s, want %s", updatedSpool.Status, spooldomain.StatusInUse)
	}

	updatedJob, err := jobRepo.GetByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetByID job error = %v", err)
	}
	if updatedJob.Status != printjobdomain.StatusPrinting {
		t.Fatalf("updatedJob.Status = %s, want %s", updatedJob.Status, printjobdomain.StatusPrinting)
	}
}

func TestSchedulerSyncRuntimeStateMarksSpoolInUseWhileJobIsActive(t *testing.T) {
	spoolRepo := memory.NewRepository()
	jobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	spoolEntity := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "eSUN", 500, 30)
	spoolEntity.CurrentWeight = 420
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), spoolEntity.ID, 180)
	job.Status = printjobdomain.StatusPrinting
	job.Progress = 32
	if err := jobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create print job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, jobRepo, notifier, 0)
	scheduler.syncRuntimeState(context.Background())

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID spool error = %v", err)
	}
	if updatedSpool.Status != spooldomain.StatusInUse {
		t.Fatalf("updatedSpool.Status = %s, want %s", updatedSpool.Status, spooldomain.StatusInUse)
	}
}

func TestSchedulerSyncRuntimeStateResetsStatusAfterPrintFinished(t *testing.T) {
	spoolRepo := memory.NewRepository()
	jobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	spoolEntity := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "eSUN", 500, 30)
	spoolEntity.CurrentWeight = 250
	spoolEntity.Status = spooldomain.StatusInUse
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), spoolEntity.ID, 180)
	job.Status = printjobdomain.StatusCompleted
	job.Progress = 100
	job.ConsumedWeight = 180
	if err := jobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create completed job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, jobRepo, notifier, 0)
	scheduler.syncRuntimeState(context.Background())

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID spool error = %v", err)
	}
	if updatedSpool.Status != spooldomain.StatusAvailable {
		t.Fatalf("updatedSpool.Status = %s, want %s", updatedSpool.Status, spooldomain.StatusAvailable)
	}
}

func TestSchedulerSyncRuntimeStateMarksLowAfterFinishedPrint(t *testing.T) {
	spoolRepo := memory.NewRepository()
	jobRepo := memory.NewPrintJobRepository()
	notifier := notificationusecase.NewService()

	spoolEntity := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "eSUN", 500, 30)
	spoolEntity.CurrentWeight = 150
	spoolEntity.Status = spooldomain.StatusInUse
	if err := spoolRepo.Create(context.Background(), spoolEntity); err != nil {
		t.Fatalf("Create spool error = %v", err)
	}

	job := printjobdomain.NewPrintJob(uuid.New(), uuid.New(), spoolEntity.ID, 180)
	job.Status = printjobdomain.StatusCompleted
	job.Progress = 100
	job.ConsumedWeight = 180
	if err := jobRepo.Create(context.Background(), job); err != nil {
		t.Fatalf("Create completed job error = %v", err)
	}

	scheduler := NewSchedulerWithDeps(spoolRepo, jobRepo, notifier, 0)
	scheduler.syncRuntimeState(context.Background())

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID spool error = %v", err)
	}
	if updatedSpool.Status != spooldomain.StatusLow {
		t.Fatalf("updatedSpool.Status = %s, want %s", updatedSpool.Status, spooldomain.StatusLow)
	}
}

func TestSchedulerRunUsesTicker(t *testing.T) {
	scheduler := NewSchedulerWithDeps(nil, nil, notificationusecase.NewService(), 0)
	scheduler.tick = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := scheduler.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
