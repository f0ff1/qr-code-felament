package runtime

import (
	"context"
	"fmt"
	"math"
	"time"

	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type spoolRepository interface {
	List(context.Context) ([]spooldomain.Spool, error)
	Update(context.Context, spooldomain.Spool) error
}

type printerRepository interface {
	List(context.Context) ([]printerdomain.Printer, error)
	Update(context.Context, printerdomain.Printer) error
}

type jobRepository interface {
	List(context.Context) ([]printjobdomain.PrintJob, error)
	Update(context.Context, printjobdomain.PrintJob) error
}

type productRepository interface {
	GetByID(context.Context, uuid.UUID) (productdomain.Product, error)
}

type cacheWriter interface {
	WriteJSON(context.Context, string, any) error
}

// Synchronizer calculates print progress on the server and persists all derived state.
type Synchronizer struct {
	spools   spoolRepository
	printers printerRepository
	jobs     jobRepository
	products productRepository
	cache    cacheWriter
	now      func() time.Time
}

func NewSynchronizer(spools spoolRepository, printers printerRepository, jobs jobRepository, products productRepository, cache cacheWriter) *Synchronizer {
	return &Synchronizer{spools: spools, printers: printers, jobs: jobs, products: products, cache: cache, now: time.Now}
}

func (s *Synchronizer) Sync(ctx context.Context) error {
	jobs, err := s.jobs.List(ctx)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}
	now := s.now()
	productCache := make(map[uuid.UUID]productdomain.Product)
	changed := false
	for i := range jobs {
		job := &jobs[i]
		if job.Status != printjobdomain.StatusPrinting {
			continue
		}
		product, ok := productCache[job.ProductID]
		if !ok {
			product, err = s.products.GetByID(ctx, job.ProductID)
			if err != nil || product.EstimatedPrintTime <= 0 {
				continue
			}
			productCache[job.ProductID] = product
		}
		progress := math.Min(100, math.Max(0, now.Sub(job.StartedAt).Seconds()/product.EstimatedPrintTime.Seconds()*100))
		if progress >= 100 {
			job.Complete()
			job.FinishedAt = &now
			changed = true
		} else if math.Abs(job.Progress-progress) >= 0.5 {
			job.Progress = progress
			job.UpdatedAt = now
			changed = true
		} else {
			continue
		}
		if err := s.jobs.Update(ctx, *job); err != nil {
			return fmt.Errorf("update job %s: %w", job.ID, err)
		}
	}
	if changed {
		jobs, err = s.jobs.List(ctx)
		if err != nil {
			return fmt.Errorf("reload jobs: %w", err)
		}
	}
	if err := s.syncPrinters(ctx, jobs, now); err != nil {
		return err
	}
	if err := s.syncSpools(ctx, jobs, now); err != nil {
		return err
	}
	for _, job := range jobs {
		if isActive(job.Status) || (job.FinishedAt != nil && now.Sub(*job.FinishedAt) < 2*time.Minute) {
			s.writeCache(ctx, "runtime:print-job:"+job.ID.String(), job)
		}
	}
	return nil
}

func (s *Synchronizer) syncPrinters(ctx context.Context, jobs []printjobdomain.PrintJob, now time.Time) error {
	printers, err := s.printers.List(ctx)
	if err != nil {
		return fmt.Errorf("list printers: %w", err)
	}
	for _, printer := range printers {
		status := printerdomain.StatusIdle
		for _, job := range jobs {
			if job.PrinterID != printer.ID {
				continue
			}
			if job.Status == printjobdomain.StatusPrinting || job.Status == printjobdomain.StatusQueued {
				status = printerdomain.StatusPrinting
				break
			}
			if job.Status == printjobdomain.StatusPaused {
				status = printerdomain.StatusPaused
			}
		}
		if printer.Status != status {
			printer.Status, printer.UpdatedAt = status, now
			if err := s.printers.Update(ctx, printer); err != nil {
				return fmt.Errorf("update printer %s: %w", printer.ID, err)
			}
		}
		s.writeCache(ctx, "runtime:printer:"+printer.ID.String(), printer)
	}
	return nil
}

func (s *Synchronizer) syncSpools(ctx context.Context, jobs []printjobdomain.PrintJob, now time.Time) error {
	spools, err := s.spools.List(ctx)
	if err != nil {
		return fmt.Errorf("list spools: %w", err)
	}
	for _, spool := range spools {
		inUse := false
		for _, job := range jobs {
			if job.SpoolID == spool.ID && isActive(job.Status) {
				inUse = true
				break
			}
		}
		status := spool.EffectiveStatus(inUse)
		if spool.Status != status {
			spool.Status, spool.UpdatedAt = status, now
			if err := s.spools.Update(ctx, spool); err != nil {
				return fmt.Errorf("update spool %s: %w", spool.ID, err)
			}
		}
		s.writeCache(ctx, "runtime:spool:"+spool.ID.String(), spool)
	}
	return nil
}

func isActive(status printjobdomain.Status) bool {
	return status == printjobdomain.StatusQueued || status == printjobdomain.StatusPrinting || status == printjobdomain.StatusPaused
}

func (s *Synchronizer) writeCache(ctx context.Context, key string, value any) {
	if s.cache != nil {
		_ = s.cache.WriteJSON(ctx, key, value)
	}
}
