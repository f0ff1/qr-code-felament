package inventory

import (
	"context"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	invdomain "filamenttracker/internal/domain/inventory"
	spooldomain "filamenttracker/internal/domain/spool"
	spoolusecase "filamenttracker/internal/usecase/spool"

	"github.com/google/uuid"
)

type InventoryRepository interface {
	Add(ctx context.Context, tx invdomain.Transaction) error
	List(ctx context.Context) ([]invdomain.Transaction, error)
}

type Service struct {
	spoolRepo spooldomain.Repository
	invRepo   InventoryRepository
}

func NewService(spoolRepo spooldomain.Repository, invRepo InventoryRepository) *Service {
	return &Service{spoolRepo: spoolRepo, invRepo: invRepo}
}

func (s *Service) CreateSpool(ctx context.Context, material spooldomain.Material, color, manufacturer string, initialWeight int, price float64) (spooldomain.Spool, error) {
	spoolService := spoolusecase.NewService(s.spoolRepo)
	return spoolService.Create(ctx, material, color, manufacturer, initialWeight, price)
}

func (s *Service) RecordConsumption(ctx context.Context, spoolID uuid.UUID, weight int) error {
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return err
	}
	if weight <= 0 {
		return fmt.Errorf("%w: weight must be positive", domain.ErrInvalid)
	}
	if spoolEntity.CurrentWeight < weight {
		return fmt.Errorf("%w: spool does not have enough weight", domain.ErrInvalid)
	}

	spoolEntity.Consume(weight)
	if err := s.spoolRepo.Update(ctx, spoolEntity); err != nil {
		return err
	}

	entry := invdomain.Transaction{
		ID:        uuid.NewString(),
		SpoolID:   spoolID.String(),
		Type:      "CONSUMPTION",
		Weight:    -weight,
		CreatedAt: time.Now(),
	}
	return s.invRepo.Add(ctx, entry)
}

func (s *Service) ListTransactions(ctx context.Context) ([]invdomain.Transaction, error) {
	return s.invRepo.List(ctx)
}

func (s *Service) GetSummary(ctx context.Context) ([]invdomain.MaterialSummary, error) {
	spools, err := s.spoolRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	summary := make(map[string]invdomain.MaterialSummary)
	for _, spoolEntity := range spools {
		key := string(spoolEntity.Material) + ":" + spoolEntity.Color
		current, ok := summary[key]
		if !ok {
			current = invdomain.MaterialSummary{Material: string(spoolEntity.Material), Color: spoolEntity.Color, Total: 0}
		}
		current.Total += spoolEntity.CurrentWeight
		summary[key] = current
	}

	res := make([]invdomain.MaterialSummary, 0, len(summary))
	for _, item := range summary {
		res = append(res, item)
	}
	return res, nil
}
