package memory

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"
	productdomain "filamenttracker/internal/domain/product"

	"github.com/google/uuid"
)

type ProductRepository struct {
	items map[uuid.UUID]productdomain.Product
}

func NewProductRepository() *ProductRepository {
	return &ProductRepository{items: make(map[uuid.UUID]productdomain.Product)}
}

func (r *ProductRepository) Create(ctx context.Context, p productdomain.Product) error {
	if _, exists := r.items[p.ID]; exists {
		return fmt.Errorf("%w: product already exists", domain.ErrConflict)
	}
	if p.SiteID == uuid.Nil {
		p.SiteID = org.DefaultSiteID
	}
	r.items[p.ID] = p
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error) {
	p, ok := r.items[id]
	if !ok {
		return productdomain.Product{}, fmt.Errorf("%w: product %s", domain.ErrNotFound, id)
	}
	return p, nil
}

func (r *ProductRepository) List(ctx context.Context) ([]productdomain.Product, error) {
	list := make([]productdomain.Product, 0, len(r.items))
	for _, p := range r.items {
		list = append(list, p)
	}
	return list, nil
}

func (r *ProductRepository) Update(ctx context.Context, p productdomain.Product) error {
	if _, exists := r.items[p.ID]; !exists {
		return fmt.Errorf("%w: product %s", domain.ErrNotFound, p.ID)
	}
	r.items[p.ID] = p
	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("%w: product %s", domain.ErrNotFound, id)
	}
	delete(r.items, id)
	return nil
}
