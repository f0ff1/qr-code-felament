package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"

	"github.com/google/uuid"
)

type OrganizationRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]org.Organization
}

func NewOrganizationRepository() *OrganizationRepository {
	now := time.Now().UTC()
	r := &OrganizationRepository{items: make(map[uuid.UUID]org.Organization)}
	r.items[org.DefaultOrganizationID] = org.Organization{
		ID:        org.DefaultOrganizationID,
		Name:      "Default Company",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return r
}

func (r *OrganizationRepository) Upsert(ctx context.Context, o org.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if existing, ok := r.items[o.ID]; ok {
		o.CreatedAt = existing.CreatedAt
	} else if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	o.UpdatedAt = now
	r.items[o.ID] = o
	return nil
}

func (r *OrganizationRepository) Update(ctx context.Context, o org.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[o.ID]; !ok {
		return fmt.Errorf("%w: organization %s", domain.ErrNotFound, o.ID)
	}
	r.items[o.ID] = o
	return nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (org.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	if !ok {
		return org.Organization{}, fmt.Errorf("%w: organization %s", domain.ErrNotFound, id)
	}
	return item, nil
}

func (r *OrganizationRepository) FindByName(ctx context.Context, name string) (org.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	needle := strings.ToLower(strings.TrimSpace(name))
	for _, item := range r.items {
		if strings.ToLower(item.Name) == needle {
			return item, nil
		}
	}
	return org.Organization{}, fmt.Errorf("%w: organization %q", domain.ErrNotFound, name)
}

func (r *OrganizationRepository) GetDefault(ctx context.Context) (org.Organization, error) {
	return r.GetByID(ctx, org.DefaultOrganizationID)
}

func (r *OrganizationRepository) EnsureDefault(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[org.DefaultOrganizationID]; ok {
		return nil
	}
	now := time.Now().UTC()
	r.items[org.DefaultOrganizationID] = org.Organization{
		ID:        org.DefaultOrganizationID,
		Name:      "Default Company",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}
