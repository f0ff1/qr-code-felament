package printer

import (
	"context"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, p printerdomain.Printer) error
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
	List(ctx context.Context) ([]printerdomain.Printer, error)
	Update(ctx context.Context, p printerdomain.Printer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo    Repository
	adapter printerdomain.Adapter
}

func NewService(repo Repository, adapter printerdomain.Adapter) *Service {
	return &Service{repo: repo, adapter: adapter}
}

func (s *Service) Create(ctx context.Context, name, model string) (printerdomain.Printer, error) {
	if name == "" || model == "" {
		return printerdomain.Printer{}, fmt.Errorf("%w: printer name and model are required", domain.ErrInvalid)
	}

	p := printerdomain.NewPrinter(name, model)
	if err := s.repo.Create(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]printerdomain.Printer, error) {
	return s.repo.List(ctx)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) SyncStatus(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	status, err := s.adapter.GetStatus(ctx, p)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	p.Status = status
	p.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetProgress(ctx context.Context, id uuid.UUID) (float64, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	return s.adapter.GetProgress(ctx, p)
}
