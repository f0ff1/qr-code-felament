package printjob

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

type PrintJobRepository interface {
	Create(ctx context.Context, j printjobdomain.PrintJob) error
	GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error)
	GetByExternalTaskID(ctx context.Context, printerID uuid.UUID, externalTaskID string) (printjobdomain.PrintJob, error)
	List(ctx context.Context) ([]printjobdomain.PrintJob, error)
	Update(ctx context.Context, j printjobdomain.PrintJob) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo        PrintJobRepository
	spoolRepo   spooldomain.Repository
	productRepo ProductRepository
	printerRepo PrinterRepository
}

type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error)
	List(ctx context.Context) ([]productdomain.Product, error)
	Create(ctx context.Context, p productdomain.Product) error
	Update(ctx context.Context, p productdomain.Product) error
}

type PrinterRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
}

type BambuSnapshot struct {
	ExternalTaskID       string
	FileName             string
	Progress             float64
	Status               printjobdomain.Status
	RemainingMin         int
	EstimatedDurationSec int
	EstimatedWeight      int
	LayerCurrent         int
	LayerTotal           int
	MaterialHint         string
	ColorHint            string
	BrandHint            string
}

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

	need := productEntity.EstimatedWeight
	reserved, err := s.reserveAvailable(ctx, &spoolEntity, need)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job := printjobdomain.NewPrintJobForSite(printerEntity.SiteID, printerEntity.ID, productEntity.ID, spoolEntity.ID, need)
	job.Start()
	job.ConsumedWeight = reserved
	job.FileName = productEntity.Name
	if err := s.repo.Create(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolEntity.ID, reserved)
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) ConfirmDraft(ctx context.Context, jobID, productID, spoolID uuid.UUID) (printjobdomain.PrintJob, error) {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	if !job.IsDraft {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: job is not a draft", domain.ErrInvalid)
	}
	productEntity, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	weight := productEntity.EstimatedWeight
	if weight <= 0 {
		weight = job.EstimatedWeight
	}
	if weight <= 0 {
		weight = 50
	}

	reserved, err := s.reserveAvailable(ctx, &spoolEntity, weight)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job.ProductID = productID
	job.SpoolID = spoolID
	job.EstimatedWeight = weight
	job.ConsumedWeight = reserved
	job.IsDraft = false
	if job.Status == printjobdomain.StatusDraft {
		job.Status = printjobdomain.StatusPrinting
	}
	job.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolID, reserved)
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func isTerminal(status printjobdomain.Status) bool {
	return status == printjobdomain.StatusCompleted || status == printjobdomain.StatusFailed || status == printjobdomain.StatusCancelled
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

	if !isTerminal(job.Status) && job.ConsumedWeight > 0 {
		unused := unusedReservedGrams(job)
		if unused > 0 {
			if err := s.refundFilament(ctx, job.SpoolID, unused); err != nil {
				return err
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
