package printjob

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type PrintJobRepository interface {
	Create(ctx context.Context, j printjobdomain.PrintJob) error
	GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error)
	List(ctx context.Context) ([]printjobdomain.PrintJob, error)
	Update(ctx context.Context, j printjobdomain.PrintJob) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo      PrintJobRepository
	spoolRepo spooldomain.Repository
	productRepo ProductRepository
	printerRepo PrinterRepository
}

type ProductRepository interface { GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error) }
type PrinterRepository interface { GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) }

func NewService(repo PrintJobRepository, spoolRepo spooldomain.Repository, productRepo ProductRepository, printerRepo PrinterRepository) *Service {
	return &Service{repo: repo, spoolRepo: spoolRepo, productRepo: productRepo, printerRepo: printerRepo}
}

func (s *Service) Start(ctx context.Context, printerID, productID, spoolID uuid.UUID) (printjobdomain.PrintJob, error) {
	printerEntity, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	productEntity, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	if productEntity.EstimatedWeight > spoolEntity.CurrentWeight {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: insufficient filament for product", domain.ErrInvalid)
	}

	spoolEntity.Consume(productEntity.EstimatedWeight)
	if err := s.spoolRepo.Update(ctx, spoolEntity); err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job := printjobdomain.NewPrintJob(printerEntity.ID, productEntity.ID, spoolEntity.ID, productEntity.EstimatedWeight)
	job.Start()
	job.ConsumedWeight = productEntity.EstimatedWeight
	if err := s.repo.Create(ctx, job); err != nil {
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) Pause(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	job.Pause()
	return s.repo.Update(ctx, job)
}

func (s *Service) Resume(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	job.Resume()
	return s.repo.Update(ctx, job)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if job.Status != printjobdomain.StatusCompleted && job.ConsumedWeight > 0 {
		spoolEntity, err := s.spoolRepo.GetByID(ctx, job.SpoolID)
		if err == nil {
			spoolEntity.Refill(job.ConsumedWeight)
			if updateErr := s.spoolRepo.Update(ctx, spoolEntity); updateErr != nil {
				return updateErr
			}
		}
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]printjobdomain.PrintJob, error) {
	return s.repo.List(ctx)
}
