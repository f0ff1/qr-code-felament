package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"
	sitedomain "filamenttracker/internal/domain/site"

	"github.com/google/uuid"
)

type SiteRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]sitedomain.Site
}

func NewSiteRepository() *SiteRepository {
	now := time.Now().UTC()
	r := &SiteRepository{items: make(map[uuid.UUID]sitedomain.Site)}
	r.items[org.DefaultSiteID] = sitedomain.Site{
		ID:             org.DefaultSiteID,
		OrganizationID: org.DefaultOrganizationID,
		Name:           "Основной склад",
		Address:        "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return r
}

func (r *SiteRepository) Create(ctx context.Context, s sitedomain.Site) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[s.ID]; exists {
		return fmt.Errorf("%w: site already exists", domain.ErrConflict)
	}
	r.items[s.ID] = s
	return nil
}

func (r *SiteRepository) Update(ctx context.Context, s sitedomain.Site) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[s.ID]; !exists {
		return fmt.Errorf("%w: site %s", domain.ErrNotFound, s.ID)
	}
	r.items[s.ID] = s
	return nil
}

func (r *SiteRepository) ListByOrg(ctx context.Context, organizationID uuid.UUID) ([]sitedomain.Site, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]sitedomain.Site, 0)
	for _, s := range r.items {
		if s.OrganizationID == organizationID {
			list = append(list, s)
		}
	}
	return list, nil
}

func (r *SiteRepository) ListAll(ctx context.Context) ([]sitedomain.Site, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]sitedomain.Site, 0, len(r.items))
	for _, s := range r.items {
		list = append(list, s)
	}
	return list, nil
}

func (r *SiteRepository) GetByID(ctx context.Context, id uuid.UUID) (sitedomain.Site, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.items[id]
	if !ok {
		return sitedomain.Site{}, fmt.Errorf("%w: site %s", domain.ErrNotFound, id)
	}
	return s, nil
}

func (r *SiteRepository) EnsureDefault(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[org.DefaultSiteID]; ok {
		return nil
	}
	now := time.Now().UTC()
	r.items[org.DefaultSiteID] = sitedomain.Site{
		ID:             org.DefaultSiteID,
		OrganizationID: org.DefaultOrganizationID,
		Name:           "Основной склад",
		Address:        "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return nil
}
