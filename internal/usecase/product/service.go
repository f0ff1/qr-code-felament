package product

import (
	"context"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type ProductRepository interface {
	Create(ctx context.Context, p productdomain.Product) error
	GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error)
	List(ctx context.Context) ([]productdomain.Product, error)
	Update(ctx context.Context, p productdomain.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo ProductRepository
}

func NewService(repo ProductRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name, description, material string, estimatedWeight int, estimatedPrintTime time.Duration, price float64) (productdomain.Product, error) {
	if name == "" || material == "" || estimatedWeight <= 0 {
		return productdomain.Product{}, fmt.Errorf("%w: invalid product input", domain.ErrInvalid)
	}

	p := productdomain.NewProduct(name, description, material, estimatedWeight, estimatedPrintTime, price)
	if err := s.repo.Create(ctx, p); err != nil {
		return productdomain.Product{}, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]productdomain.Product, error) {
	return s.repo.List(ctx)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ValidateNeed(ctx context.Context, requiredWeight int, spool spooldomain.Spool) error {
	if requiredWeight > spool.CurrentWeight {
		return fmt.Errorf("%w: insufficient filament for this product", domain.ErrInvalid)
	}
	return nil
}

func (s *Service) StartPrintJob(
	ctx context.Context,
	printerRepo interface {
		GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
	},
	spoolRepo interface {
		GetByID(ctx context.Context, id uuid.UUID) (spooldomain.Spool, error)
	},
	printerID, productID, spoolID uuid.UUID,
) (printjobdomain.PrintJob, error) {
	printerEntity, err := printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	productEntity, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	spoolEntity, err := spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	if err := s.ValidateNeed(ctx, productEntity.EstimatedWeight, spoolEntity); err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job := printjobdomain.NewPrintJob(printerEntity.ID, productEntity.ID, spoolEntity.ID, productEntity.EstimatedWeight)
	job.Start()
	return job, nil
}
