package printer

import (
	"context"
	"testing"

	printerdomain "filamenttracker/internal/domain/printer"
	"filamenttracker/internal/infrastructure/printer/mock"
	"filamenttracker/internal/repository/memory"
)

func TestServiceCreateAndGetByID(t *testing.T) {
	repo := memory.NewPrinterRepository()
	service := NewService(repo, mock.Adapter{})

	printer, err := service.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if printer.Name != "Bambu A1" {
		t.Fatalf("printer.Name = %s, want Bambu A1", printer.Name)
	}

	got, err := service.GetByID(context.Background(), printer.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != printer.ID {
		t.Fatalf("GetByID() returned wrong ID")
	}
}

func TestServiceSyncStatusUpdatesPrinter(t *testing.T) {
	repo := memory.NewPrinterRepository()
	service := NewService(repo, mock.Adapter{})

	printer, err := service.Create(context.Background(), "Ender 3", "Creality")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := service.SyncStatus(context.Background(), printer.ID)
	if err != nil {
		t.Fatalf("SyncStatus() error = %v", err)
	}
	if updated.Status == "" {
		t.Fatal("SyncStatus() did not update status")
	}
}

func TestServiceGetProgress(t *testing.T) {
	repo := memory.NewPrinterRepository()
	service := NewService(repo, mock.Adapter{})

	printer, err := service.Create(context.Background(), "Prusa MK4", "Prusa")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	progress, err := service.GetProgress(context.Background(), printer.ID)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}
	if progress < 0 || progress > 100 {
		t.Fatalf("progress = %v, want between 0 and 100", progress)
	}
}

func TestServiceCreateRejectsEmptyInput(t *testing.T) {
	repo := memory.NewPrinterRepository()
	service := NewService(repo, mock.Adapter{})

	if _, err := service.Create(context.Background(), "", ""); err == nil {
		t.Fatal("Create() expected error for empty input")
	}
}

func TestPrinterEntityDefaultStatusIsIdle(t *testing.T) {
	p := printerdomain.NewPrinter("Printer X", "Model Y")
	if p.Status != printerdomain.StatusIdle {
		t.Fatalf("Status = %s, want %s", p.Status, printerdomain.StatusIdle)
	}
}
