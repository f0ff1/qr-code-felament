package inventory

import (
	"context"
	"testing"

	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/repository/memory"
)

func TestServiceRecordConsumptionCreatesLedgerEntry(t *testing.T) {
	spoolRepo := memory.NewRepository()
	invRepo := memory.NewInventoryRepository()
	service := NewService(spoolRepo, invRepo)

	spool, err := service.CreateSpool(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("CreateSpool() error = %v", err)
	}

	if err := service.RecordConsumption(context.Background(), spool.ID, 180); err != nil {
		t.Fatalf("RecordConsumption() error = %v", err)
	}

	ledger, err := service.ListTransactions(context.Background())
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(ledger) != 1 {
		t.Fatalf("len(transactions) = %d, want 1", len(ledger))
	}
	if ledger[0].Weight != -180 {
		t.Fatalf("transaction weight = %d, want -180", ledger[0].Weight)
	}
	if ledger[0].Type != "CONSUMPTION" {
		t.Fatalf("transaction type = %s, want CONSUMPTION", ledger[0].Type)
	}
}

func TestServiceSummaryTotalsMaterialByColor(t *testing.T) {
	spoolRepo := memory.NewRepository()
	invRepo := memory.NewInventoryRepository()
	service := NewService(spoolRepo, invRepo)

	_, err := service.CreateSpool(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30)
	if err != nil {
		t.Fatalf("CreateSpool() error = %v", err)
	}
	_, err = service.CreateSpool(context.Background(), spooldomain.MaterialPLA, "Black", "Sunlu", 500, 20)
	if err != nil {
		t.Fatalf("Second CreateSpool() error = %v", err)
	}

	summary, err := service.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if len(summary) != 1 {
		t.Fatalf("len(summary) = %d, want 1", len(summary))
	}
	if summary[0].Total != 1500 {
		t.Fatalf("total = %d, want 1500", summary[0].Total)
	}
}
