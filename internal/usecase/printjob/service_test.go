package printjob

import (
	"context"
	"testing"
	"unicode/utf8"

	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/repository/memory"
	productusecase "filamenttracker/internal/usecase/product"
)

func TestSanitizeUTF8StripsInvalidBytes(t *testing.T) {
	raw := "model" + string([]byte{0x80}) + "_v1.gcode"
	got := sanitizeUTF8(raw)
	if !utf8.ValidString(got) {
		t.Fatalf("sanitizeUTF8 returned invalid UTF-8: %q", got)
	}
	if got != "model_v1.gcode" {
		t.Fatalf("sanitizeUTF8 = %q, want model_v1.gcode", got)
	}
	name := productDisplayName(raw)
	if !utf8.ValidString(name) || name == "" {
		t.Fatalf("productDisplayName = %q", name)
	}
}

func TestSyncFromBambuAcceptsInvalidUTF8FileName(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	if err := printerRepo.Create(context.Background(), printer); err != nil {
		t.Fatal(err)
	}

	fileName := "print" + string([]byte{0x80, 0xff}) + "_box.3mf"
	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-bad-utf8",
		FileName:       fileName,
		Progress:       5,
		Status:         printjobdomain.StatusPrinting,
		RemainingMin:   60,
	})
	if err != nil {
		t.Fatalf("SyncFromBambu: %v", err)
	}
	if !created {
		t.Fatal("expected created job")
	}
	if !utf8.ValidString(job.FileName) {
		t.Fatalf("job file name invalid UTF-8: %q", job.FileName)
	}
	products, _ := productRepo.List(context.Background())
	if len(products) != 1 {
		t.Fatalf("products = %d, want 1", len(products))
	}
	if !utf8.ValidString(products[0].Name) || !utf8.ValidString(products[0].Description) {
		t.Fatalf("product strings invalid: name=%q desc=%q", products[0].Name, products[0].Description)
	}
}

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
	if job.ProductID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Fatal("expected auto-created product even for unmatched spool")
	}
	if job.RemainingMinutes != 90 {
		t.Fatalf("remaining minutes = %d, want 90", job.RemainingMinutes)
	}
	products, _ := productRepo.List(context.Background())
	if len(products) != 1 {
		t.Fatalf("products = %d, want 1 auto product", len(products))
	}
}

func TestSyncFromBambuAutoProductPriceUsesSpool(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)

	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:       "task-auto-product",
		FileName:             "bracket_v2.gcode",
		Progress:             20,
		Status:               printjobdomain.StatusPrinting,
		RemainingMin:         48,
		EstimatedDurationSec: 3600,
		EstimatedWeight:      100,
		MaterialHint:         "Generic PLA",
		ColorHint:            "Charcoal",
		BrandHint:            "Generic",
	})
	if err != nil {
		t.Fatalf("SyncFromBambu: %v", err)
	}
	if !created || job.IsDraft || job.ProductID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected linked auto product, got draft=%v product=%s", job.IsDraft, job.ProductID)
	}
	product, err := productRepo.GetByID(context.Background(), job.ProductID)
	if err != nil {
		t.Fatal(err)
	}
	if product.Name == "" || product.Price <= 0 {
		t.Fatalf("auto product missing name/price: %+v", product)
	}
	if product.EstimatedWeight != 100 {
		t.Fatalf("weight = %d, want 100", product.EstimatedWeight)
	}
}

func TestSyncFromBambuUpdatesProgressAndRemaining(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)

	_, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-progress",
		FileName:       "cube.gcode",
		Progress:       40,
		Status:         printjobdomain.StatusPrinting,
		RemainingMin:   60,
	})
	if err != nil {
		t.Fatal(err)
	}

	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID: "task-progress",
		FileName:       "cube.gcode",
		Progress:       35,
		Status:         printjobdomain.StatusPaused,
		RemainingMin:   66,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("should update existing job")
	}
	if job.Progress != 35 {
		t.Fatalf("progress = %v, want 35 (printer trust)", job.Progress)
	}
	if job.RemainingMinutes != 66 {
		t.Fatalf("remaining = %d, want 66", job.RemainingMinutes)
	}
	if job.Status != printjobdomain.StatusPaused {
		t.Fatalf("status = %s, want paused (Cloud status kept even when unmatched)", job.Status)
	}
	if !job.IsDraft {
		t.Fatal("unmatched spool should keep is_draft=true")
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
	product, err := products.Create(context.Background(), "Bracket", "", "PLA", 120, 0, 10, 0, productdomain.BillingPerson)
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

func TestSyncFromBambuReservesWeightAndUpdatesLayers(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)

	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-weight-layers",
		FileName:        "tower.gcode",
		Progress:        10,
		Status:          printjobdomain.StatusPrinting,
		RemainingMin:    90,
		EstimatedWeight: 200,
		LayerCurrent:    9,
		LayerTotal:      90,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || job.IsDraft {
		t.Fatalf("expected reserved non-draft job, got draft=%v", job.IsDraft)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 800 {
		t.Fatalf("future remaining = %d, want 800", updatedSpool.CurrentWeight)
	}
	if job.LayerCurrent != 9 || job.LayerTotal != 90 {
		t.Fatalf("layers = %d/%d, want 9/90", job.LayerCurrent, job.LayerTotal)
	}

	job, _, err = service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-weight-layers",
		FileName:        "tower.gcode",
		Progress:        50,
		Status:          printjobdomain.StatusPrinting,
		EstimatedWeight: 240,
		LayerCurrent:    45,
		LayerTotal:      90,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedSpool, _ = spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 760 {
		t.Fatalf("future remaining after weight bump = %d, want 760", updatedSpool.CurrentWeight)
	}
	if job.EstimatedWeight != 240 || job.LayerCurrent != 45 {
		t.Fatalf("job weight/layers = %d %d/%d", job.EstimatedWeight, job.LayerCurrent, job.LayerTotal)
	}
	product, _ := productRepo.GetByID(context.Background(), job.ProductID)
	if product.EstimatedWeight != 240 {
		t.Fatalf("auto product weight = %d, want 240", product.EstimatedWeight)
	}
}

func TestSyncFromBambuDoesNotTreatStale100AsFullyConsumed(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)

	job, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-stale-100",
		FileName:        "bunny.gcode",
		Progress:        100,
		Status:          printjobdomain.StatusPrinting,
		RemainingMin:    55,
		EstimatedWeight: 100,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Progress >= 100 {
		t.Fatalf("live job with ETA must not keep progress=100, got %v", job.Progress)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 900 {
		t.Fatalf("future remaining = %d, want 900", updatedSpool.CurrentWeight)
	}
}

func TestApplyLayersDoesNotInventFromProgress(t *testing.T) {
	job := printjobdomain.PrintJob{LayerTotal: 750}
	changed := applyLayers(&job, BambuSnapshot{
		Progress:   2,
		LayerTotal: 750,
		Status:     printjobdomain.StatusPrinting,
	})
	if job.LayerCurrent != 0 {
		t.Fatalf("invented layer current=%d from 2%%, want 0", job.LayerCurrent)
	}
	if !changed && job.LayerTotal != 750 {
		t.Fatal("expected total layers to apply")
	}
}

func TestEnsureDraftLinkedReservesDefaultSpool(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)
	printer := printerdomain.NewPrinter("A1", "A1")
	printer.DefaultSpoolID = spool.ID
	_ = printerRepo.Create(context.Background(), printer)

	// First sync: no material hints → historically stayed draft with no reserve.
	job, created, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-default-spool",
		FileName:        "remote.gcode",
		Progress:        2,
		Status:          printjobdomain.StatusPrinting,
		RemainingMin:    100,
		EstimatedWeight: 150,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created")
	}
	if job.IsDraft || job.SpoolID != spool.ID || job.ConsumedWeight != 150 {
		t.Fatalf("default spool must auto-link+reserve, got draft=%v spool=%s consumed=%d", job.IsDraft, job.SpoolID, job.ConsumedWeight)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 850 {
		t.Fatalf("future remaining = %d, want 850", updatedSpool.CurrentWeight)
	}
}

func TestSyncFromBambuReservesMissingConsumedWeight(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)

	job, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-fix-reserve",
		FileName:        "box.gcode",
		Progress:        5,
		Status:          printjobdomain.StatusPrinting,
		EstimatedWeight: 120,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate broken state: linked job without spool reservation.
	job.ConsumedWeight = 0
	spool.CurrentWeight = 1000
	_ = spoolRepo.Update(context.Background(), spool)
	_ = jobRepo.Update(context.Background(), job)

	job, _, err = service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-fix-reserve",
		FileName:        "box.gcode",
		Progress:        8,
		Status:          printjobdomain.StatusPrinting,
		EstimatedWeight: 120,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ConsumedWeight != 120 {
		t.Fatalf("consumed=%d, want 120", job.ConsumedWeight)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 880 {
		t.Fatalf("future remaining = %d, want 880", updatedSpool.CurrentWeight)
	}
}

func TestCancelRefundsOnlyUnusedFilament(t *testing.T) {
	spoolRepo := memory.NewRepository()
	printerRepo := memory.NewPrinterRepository()
	productRepo := memory.NewProductRepository()
	jobRepo := memory.NewPrintJobRepository()
	service := NewService(jobRepo, spoolRepo, productRepo, printerRepo)

	printer := printerdomain.NewPrinter("A1", "A1")
	_ = printerRepo.Create(context.Background(), printer)
	spool := spooldomain.NewSpool(spooldomain.MaterialPLA, "чёрный", "Generic", 1000, 40)
	_ = spoolRepo.Create(context.Background(), spool)

	job, _, err := service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-cancel-partial",
		FileName:        "part.gcode",
		Progress:        0.5,
		Status:          printjobdomain.StatusPrinting,
		EstimatedWeight: 200,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedSpool, _ := spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 800 {
		t.Fatalf("after reserve = %d, want 800", updatedSpool.CurrentWeight)
	}

	// ~0.5% of 200g ≈ 1g used → refund 199 → spool 999
	job, _, err = service.SyncFromBambu(context.Background(), printer, BambuSnapshot{
		ExternalTaskID:  "task-cancel-partial",
		FileName:        "part.gcode",
		Progress:        0.5,
		Status:          printjobdomain.StatusCancelled,
		EstimatedWeight: 200,
		MaterialHint:    "Generic PLA",
		ColorHint:       "Charcoal",
		BrandHint:       "Generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != printjobdomain.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", job.Status)
	}
	if job.ConsumedWeight != 1 {
		t.Fatalf("consumed after cancel = %d, want 1 (used portion)", job.ConsumedWeight)
	}
	updatedSpool, _ = spoolRepo.GetByID(context.Background(), spool.ID)
	if updatedSpool.CurrentWeight != 999 {
		t.Fatalf("after cancel remaining = %d, want 999", updatedSpool.CurrentWeight)
	}
}

func TestUnusedReservedGrams(t *testing.T) {
	if got := unusedReservedGrams(printjobdomain.PrintJob{ConsumedWeight: 200, Progress: 50}); got != 100 {
		t.Fatalf("50%% of 200 => unused %d, want 100", got)
	}
	if got := unusedReservedGrams(printjobdomain.PrintJob{ConsumedWeight: 200, Progress: 0}); got != 200 {
		t.Fatalf("0%% => unused %d, want 200", got)
	}
	if got := unusedReservedGrams(printjobdomain.PrintJob{ConsumedWeight: 200, Progress: 100}); got != 0 {
		t.Fatalf("100%% => unused %d, want 0", got)
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
