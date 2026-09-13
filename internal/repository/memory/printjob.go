package memory

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"
	printjobdomain "filamenttracker/internal/domain/printjob"

	"github.com/google/uuid"
)

type PrintJobRepository struct {
	items map[uuid.UUID]printjobdomain.PrintJob
}

func NewPrintJobRepository() *PrintJobRepository {
	return &PrintJobRepository{items: make(map[uuid.UUID]printjobdomain.PrintJob)}
}

func (r *PrintJobRepository) Create(ctx context.Context, j printjobdomain.PrintJob) error {
	if _, exists := r.items[j.ID]; exists {
		return fmt.Errorf("%w: print job already exists", domain.ErrConflict)
	}
	if j.SiteID == uuid.Nil {
		j.SiteID = org.DefaultSiteID
	}
	r.items[j.ID] = j
	return nil
}

func (r *PrintJobRepository) GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error) {
	j, ok := r.items[id]
	if !ok {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: print job %s", domain.ErrNotFound, id)
	}
	return j, nil
}

func (r *PrintJobRepository) GetByExternalTaskID(ctx context.Context, printerID uuid.UUID, externalTaskID string) (printjobdomain.PrintJob, error) {
	for _, j := range r.items {
		if j.PrinterID == printerID && j.ExternalTaskID == externalTaskID {
			return j, nil
		}
	}
	return printjobdomain.PrintJob{}, fmt.Errorf("%w: print job external %s", domain.ErrNotFound, externalTaskID)
}

func (r *PrintJobRepository) List(ctx context.Context) ([]printjobdomain.PrintJob, error) {
	list := make([]printjobdomain.PrintJob, 0, len(r.items))
	for _, j := range r.items {
		list = append(list, j)
	}
	return list, nil
}

func (r *PrintJobRepository) Update(ctx context.Context, j printjobdomain.PrintJob) error {
	if _, exists := r.items[j.ID]; !exists {
		return fmt.Errorf("%w: print job %s", domain.ErrNotFound, j.ID)
	}
	r.items[j.ID] = j
	return nil
}

func (r *PrintJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("%w: print job %s", domain.ErrNotFound, id)
	}
	delete(r.items, id)
	return nil
}
