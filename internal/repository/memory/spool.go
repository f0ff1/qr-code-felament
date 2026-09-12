package memory

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type Repository struct {
	items map[uuid.UUID]spooldomain.Spool
}

func NewRepository() *Repository {
	return &Repository{items: make(map[uuid.UUID]spooldomain.Spool)}
}

func (r *Repository) Create(ctx context.Context, s spooldomain.Spool) error {
	if _, exists := r.items[s.ID]; exists {
		return fmt.Errorf("%w: spool already exists", domain.ErrConflict)
	}
	r.items[s.ID] = s
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (spooldomain.Spool, error) {
	item, ok := r.items[id]
	if !ok {
		return spooldomain.Spool{}, fmt.Errorf("%w: spool %s", domain.ErrNotFound, id)
	}
	return item, nil
}

func (r *Repository) GetByQRToken(ctx context.Context, token string) (spooldomain.Spool, error) {
	for _, item := range r.items {
		if item.QRToken == token {
			return item, nil
		}
	}
	return spooldomain.Spool{}, fmt.Errorf("%w: spool token %s", domain.ErrNotFound, token)
}

func (r *Repository) List(ctx context.Context) ([]spooldomain.Spool, error) {
	list := make([]spooldomain.Spool, 0, len(r.items))
	for _, item := range r.items {
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) Update(ctx context.Context, s spooldomain.Spool) error {
	if _, exists := r.items[s.ID]; !exists {
		return fmt.Errorf("%w: spool %s", domain.ErrNotFound, s.ID)
	}
	r.items[s.ID] = s
	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, exists := r.items[id]; !exists {
		return fmt.Errorf("%w: spool %s", domain.ErrNotFound, id)
	}
	delete(r.items, id)
	return nil
}
