package product

import (
	"context"
	"testing"
	"time"

	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/infrastructure/printer/mock"
	"filamenttracker/internal/repository/memory"
	printerusecase "filamenttracker/internal/usecase/printer"
	printjobusecase "filamenttracker/internal/usecase/printjob"
	spoolusecase "filamenttracker/internal/usecase/spool"
)

func TestProductCreateAndGetByID(t *testing.T) {
	repo := memory.NewProductRepository()
	service := NewService(repo)

	p, err := service.Create(context.Background(), "Dragon", "Large decorative vase", "PLA", 180, 4*time.Hour+30*time.Minute, 35.0)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Name != "Dragon" {
		t.Fatalf("Name = %s, want Dragon", p.Name)
	}

	got, err := service.GetByID(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("GetByID() returned wrong product")
	}
}

func TestPrintJobCanStartWhenFilamentIsEnough(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	printJobRepo := memory.NewPrintJobRepository()

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)

	spoolEntity, err := spoolService.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("Create spool error = %v", err)
	}
	printerEntity, err := printerService.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create printer error = %v", err)
	}
	productEntity, err := productService.Create(context.Background(), "Dragon", "Decor", "PLA", 180, 4*time.Hour, 35)
	if err != nil {
		t.Fatalf("Create product error = %v", err)
	}

	job, err := printJobService.Start(context.Background(), printerEntity.ID, productEntity.ID, spoolEntity.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if job.Status != printjobdomain.StatusPrinting {
		t.Fatalf("job status = %s, want %s", job.Status, printjobdomain.StatusPrinting)
	}
}

func TestPrintJobRejectsInsufficientFilament(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	printJobRepo := memory.NewPrintJobRepository()

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)

	spoolEntity, err := spoolService.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 100, 30)
	if err != nil {
		t.Fatalf("Create spool error = %v", err)
	}
	printerEntity, err := printerService.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create printer error = %v", err)
	}
	productEntity, err := productService.Create(context.Background(), "Dragon", "Decor", "PLA", 180, 4*time.Hour, 35)
	if err != nil {
		t.Fatalf("Create product error = %v", err)
	}

	if _, err := printJobService.Start(context.Background(), printerEntity.ID, productEntity.ID, spoolEntity.ID); err == nil {
		t.Fatal("Start() expected error for insufficient filament")
	}
}

func TestPrintJobStartConsumesFilament(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	printJobRepo := memory.NewPrintJobRepository()

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)

	spoolEntity, err := spoolService.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("Create spool error = %v", err)
	}
	printerEntity, err := printerService.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create printer error = %v", err)
	}
	productEntity, err := productService.Create(context.Background(), "Dragon", "Decor", "PLA", 180, 4*time.Hour, 35)
	if err != nil {
		t.Fatalf("Create product error = %v", err)
	}

	job, err := printJobService.Start(context.Background(), printerEntity.ID, productEntity.ID, spoolEntity.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if job.Status != printjobdomain.StatusPrinting {
		t.Fatalf("job status = %s, want %s", job.Status, printjobdomain.StatusPrinting)
	}

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID() spool error = %v", err)
	}
	if updatedSpool.CurrentWeight != 820 {
		t.Fatalf("spool current weight = %d, want 820", updatedSpool.CurrentWeight)
	}
}

func TestPrintJobPauseAndResume(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	printJobRepo := memory.NewPrintJobRepository()

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)

	spoolEntity, err := spoolService.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("Create spool error = %v", err)
	}
	printerEntity, err := printerService.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create printer error = %v", err)
	}
	productEntity, err := productService.Create(context.Background(), "Dragon", "Decor", "PLA", 180, 4*time.Hour, 35)
	if err != nil {
		t.Fatalf("Create product error = %v", err)
	}

	job, err := printJobService.Start(context.Background(), printerEntity.ID, productEntity.ID, spoolEntity.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := printJobService.Pause(context.Background(), job.ID); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	job, err = printJobService.GetByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if job.Status != printjobdomain.StatusPaused {
		t.Fatalf("job status after pause = %s, want %s", job.Status, printjobdomain.StatusPaused)
	}

	if err := printJobService.Resume(context.Background(), job.ID); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	job, err = printJobService.GetByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if job.Status != printjobdomain.StatusPrinting {
		t.Fatalf("job status after resume = %s, want %s", job.Status, printjobdomain.StatusPrinting)
	}
}

func TestDeleteUnfinishedJobRestoresFilament(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	printJobRepo := memory.NewPrintJobRepository()

	spoolService := spoolusecase.NewService(spoolRepo)
	printerService := printerusecase.NewService(printerRepo, mock.Adapter{})
	productService := NewService(productRepo)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)

	spoolEntity, err := spoolService.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("Create spool error = %v", err)
	}
	printerEntity, err := printerService.Create(context.Background(), "Bambu A1", "Bambu Lab")
	if err != nil {
		t.Fatalf("Create printer error = %v", err)
	}
	productEntity, err := productService.Create(context.Background(), "Dragon", "Decor", "PLA", 180, 4*time.Hour, 35)
	if err != nil {
		t.Fatalf("Create product error = %v", err)
	}

	job, err := printJobService.Start(context.Background(), printerEntity.ID, productEntity.ID, spoolEntity.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := printJobService.Delete(context.Background(), job.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	updatedSpool, err := spoolRepo.GetByID(context.Background(), spoolEntity.ID)
	if err != nil {
		t.Fatalf("GetByID() spool error = %v", err)
	}
	if updatedSpool.CurrentWeight != 1000 {
		t.Fatalf("spool current weight after delete = %d, want 1000", updatedSpool.CurrentWeight)
	}
}

func TestProductEntityDefaults(t *testing.T) {
	p := productdomain.NewProduct("Cube", "Sample", "PLA", 120, 2*time.Hour, 20)
	if p.Material != "PLA" {
		t.Fatalf("Material = %s, want PLA", p.Material)
	}
}
