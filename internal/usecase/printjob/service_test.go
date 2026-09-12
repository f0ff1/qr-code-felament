package printjob

import (
	"context"
	"testing"

	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/repository/memory"
	productusecase "filamenttracker/internal/usecase/product"
)

func TestSyncFromBambuCreatesDraftWhenUnmatched(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	if err := printerRepo.Create(context.Background(), printer); err != nil {
		t.Fatal(err)
	}

	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-1",
		FileName:       "unknown_model.gcode",
		Progress:       12,
		Status:         printjobdomain.StatusPrinting,
		RemainingMin:   90,
	})
	if err != nil {
		t.Fatalf("SyncFromBambu: %v", err)
	}
	if !created || !job.IsDraft {
		t.Fatalf("expected draft job, got created=%v draft=%v", created, job.IsDraft)
	}
}

func TestConfirmDraftReservesFilament(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)
	products := productusecase.NewService(productRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "Black", "Bambu", 1000, 30)
	_ = spoolRepo.Create(context.Background(), spool)
	product, err := products.Create(context.Background(), "Bracket", "", "PLA", 120, 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	job, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-2",
		FileName:       "misc.gcode",
		Progress:       5,
		Status:         printjobdomain.StatusPrinting,
	})
	if err != nil {
		t.Fatal(err)
	}

	confirmed, err := service.ConfirmDraft(context.Background(), job.ID, product.ID, spool.ID)
	if err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	if confirmed.IsDraft || confirmed.ConsumedWeight != 120 {
		t.Fatalf("unexpected confirm result: %+v", confirmed)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 880 {
		t.Fatalf("spool weight = %d, want 880", updatedSpool.CurrentWeight)
	}
}

func TestSyncFromBambuMatchesGenericPLACharcoal(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	// Warehouse: name ignored, manufacturer Generic, PLA, чёрный
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 30)
	_ = spoolRepo.Create(context.Background(), spool)

	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-generic-pla",
		FileName:        "part.gcode",
		Progress:        10,
		Status:          printjobdomain.StatusPrinting,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
		EstimatedWeight: 80,
	})
	if err != nil {
		t.Fatalf("SyncFromBambu: %v", err)
	}
	if !created || job.IsDraft || job.SpoolID != spool.ID {
		t.Fatalf("expected matched Generic PLA + Charcoal, got draft=%v spool=%s created=%v", job.IsDraft, job.SpoolID, created)
	}
}

func TestSyncFromBambuSkipsWrongManufacturer(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "eSUN", 1000, 30)
	_ = spoolRepo.Create(context.Background(), spool)

	job, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-wrong-brand",
		FileName:       "part.gcode",
		Progress:       10,
		Status:         printjobdomain.StatusPrinting,
		MaterialHint:   "Generic PLA",
		ColorHint:      "Charcoal",
		BrandHint:      "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !job.IsDraft {
		t.Fatalf("eSUN spool must not match Generic brand, got draft=%v", job.IsDraft)
	}
}

func TestNormalizeMaterialGenericPLA(t *testing.T) {
	if got := normalizeMaterial("Generic PLA"); got != "PLA" {
		t.Fatalf("Generic PLA => %s, want PLA", got)
	}
	material, brand := parseFilamentHint("Generic PLA", "")
	if material != "PLA" || brand != "generic" {
		t.Fatalf("parseFilamentHint => %s/%s", material, brand)
	}
}
