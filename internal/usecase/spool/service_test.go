package spool

import (
	"context"
	"testing"

	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/repository/memory"
)

func TestServiceCreateAndGetByID(t *testing.T) {
	repo := memory.NewRepository()
	service := NewService(repo)

	spool, err := service.Create(context.Background(), spooldomain.MaterialPLA, "Black", "eSUN", 1000, 30.5)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if spool.CurrentWeight != 1000 {
		t.Fatalf("CurrentWeight = %d, want 1000", spool.CurrentWeight)
	}
	if spool.Status != spooldomain.StatusAvailable {
		t.Fatalf("Status = %s, want %s", spool.Status, spooldomain.StatusAvailable)
	}

	got, err := service.GetByID(context.Background(), spool.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != spool.ID {
		t.Fatalf("GetByID() returned wrong ID")
	}
}

func TestServiceGetByQRToken(t *testing.T) {
	repo := memory.NewRepository()
	service := NewService(repo)

	spool, err := service.Create(context.Background(), spooldomain.MaterialPETG, "White", "Prusament", 500, 25)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := service.GetByQRToken(context.Background(), spool.QRToken)
	if err != nil {
		t.Fatalf("GetByQRToken() error = %v", err)
	}
	if got.ID != spool.ID {
		t.Fatal("GetByQRToken() returned wrong spool")
	}
}

func TestServiceConsume(t *testing.T) {
	repo := memory.NewRepository()
	service := NewService(repo)

	spool, err := service.Create(context.Background(), spooldomain.MaterialPETG, "White", "Prusament", 500, 25)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.Consume(context.Background(), spool.ID, 180); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	got, err := service.GetByID(context.Background(), spool.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.CurrentWeight != 320 {
		t.Fatalf("CurrentWeight after consume = %d, want 320", got.CurrentWeight)
	}
}

func TestServiceConsumeRejectsInsufficientWeight(t *testing.T) {
	repo := memory.NewRepository()
	service := NewService(repo)

	spool, err := service.Create(context.Background(), spooldomain.MaterialPLA, "Red", "Generic", 200, 20)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.Consume(context.Background(), spool.ID, 250); err == nil {
		t.Fatal("Consume() expected error for insufficient weight")
	}
}
