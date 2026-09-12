package memory

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"

	"github.com/google/uuid"
)

type PrinterRepository struct {
	items map[uuid.UUID]printerdomain.Printer
}

func NewPrinterRepository() *PrinterRepository {
	return &PrinterRepository{items: make(map[uuid.UUID]printerdomain.Printer)}
}

func (r *PrinterRepository) Create(ctx context.Context, p printerdomain.Printer) error {
	if _, exists := r.items[p.ID]; exists {
		return fmt.Errorf("%w: printer already exists", domain.ErrConflict)
	}
	r.items[p.ID] = p
	return nil
}

func (r *PrinterRepository) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	p, ok := r.items[id]
	if !ok {
		return printerdomain.Printer{}, fmt.Errorf("%w: printer %s", domain.ErrNotFound, id)
	}
	return p, nil
}

func (r *PrinterRepository) List(ctx context.Context) ([]printerdomain.Printer, error) {
	list := make([]printerdomain.Printer, 0, len(r.items))
	for _, p := range r.items {
		list = append(list, p)
	}
	return list, nil
}

func (r *PrinterRepository) Update(ctx context.Context, p printerdomain.Printer) error {
	if _, exists := r.items[p.ID]; !exists {
		return fmt.Errorf("%w: printer %s", domain.ErrNotFound, p.ID)
	}
	r.items[p.ID] = p
	return nil
}

func (r *PrinterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("%w: printer %s", domain.ErrNotFound, id)
	}
	delete(r.items, id)
	return nil
}
