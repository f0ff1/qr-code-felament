package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"filamenttracker/internal/domain"
	userdomain "filamenttracker/internal/domain/user"

	"github.com/google/uuid"
)

type UserRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]userdomain.User
	sites map[uuid.UUID][]uuid.UUID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		items: make(map[uuid.UUID]userdomain.User),
		sites: make(map[uuid.UUID][]uuid.UUID),
	}
}

type PasswordResetRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]userdomain.PasswordResetRequest
}

func NewPasswordResetRepository() *PasswordResetRepository {
	return &PasswordResetRepository{items: make(map[uuid.UUID]userdomain.PasswordResetRequest)}
}

func (r *UserRepository) Create(ctx context.Context, u userdomain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[u.ID]; exists {
		return fmt.Errorf("%w: user already exists", domain.ErrConflict)
	}
	for _, existing := range r.items {
		if strings.EqualFold(existing.Username, u.Username) {
			return fmt.Errorf("%w: username already exists", domain.ErrConflict)
		}
	}
	r.items[u.ID] = u
	if len(u.SiteIDs) > 0 {
		r.sites[u.ID] = append([]uuid.UUID(nil), u.SiteIDs...)
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u userdomain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[u.ID]; !exists {
		return fmt.Errorf("%w: user %s", domain.ErrNotFound, u.ID)
	}
	for _, existing := range r.items {
		if existing.ID != u.ID && strings.EqualFold(existing.Username, u.Username) {
			return fmt.Errorf("%w: username already exists", domain.ErrConflict)
		}
	}
	u.SiteIDs = append([]uuid.UUID(nil), r.sites[u.ID]...)
	r.items[u.ID] = u
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (userdomain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.items[id]
	if !ok {
		return userdomain.User{}, fmt.Errorf("%w: user %s", domain.ErrNotFound, id)
	}
	u.SiteIDs = append([]uuid.UUID(nil), r.sites[id]...)
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (userdomain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.items {
		if strings.EqualFold(u.Username, username) {
			u.SiteIDs = append([]uuid.UUID(nil), r.sites[u.ID]...)
			return u, nil
		}
	}
	return userdomain.User{}, fmt.Errorf("%w: user %s", domain.ErrNotFound, username)
}

func (r *UserRepository) ListByOrg(ctx context.Context, organizationID uuid.UUID) ([]userdomain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]userdomain.User, 0)
	for _, u := range r.items {
		if u.OrganizationID == organizationID {
			u.SiteIDs = append([]uuid.UUID(nil), r.sites[u.ID]...)
			list = append(list, u)
		}
	}
	return list, nil
}

func (r *UserRepository) ListAll(ctx context.Context) ([]userdomain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]userdomain.User, 0, len(r.items))
	for _, u := range r.items {
		u.SiteIDs = append([]uuid.UUID(nil), r.sites[u.ID]...)
		list = append(list, u)
	}
	return list, nil
}

func (r *UserRepository) SetSites(ctx context.Context, userID uuid.UUID, siteIDs []uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[userID]; !exists {
		return fmt.Errorf("%w: user %s", domain.ErrNotFound, userID)
	}
	clean := make([]uuid.UUID, 0, len(siteIDs))
	for _, id := range siteIDs {
		if id != uuid.Nil {
			clean = append(clean, id)
		}
	}
	r.sites[userID] = clean
	u := r.items[userID]
	u.SiteIDs = append([]uuid.UUID(nil), clean...)
	r.items[userID] = u
	return nil
}

func (r *UserRepository) GetSiteIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.items[userID]; !exists {
		return nil, fmt.Errorf("%w: user %s", domain.ErrNotFound, userID)
	}
	return append([]uuid.UUID(nil), r.sites[userID]...), nil
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items), nil
}

func (r *PasswordResetRepository) Create(ctx context.Context, req userdomain.PasswordResetRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[req.ID]; exists {
		return fmt.Errorf("%w: password reset already exists", domain.ErrConflict)
	}
	if req.Status == "" {
		req.Status = "open"
	}
	r.items[req.ID] = req
	return nil
}

func (r *PasswordResetRepository) ListOpen(ctx context.Context) ([]userdomain.PasswordResetRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]userdomain.PasswordResetRequest, 0)
	for _, req := range r.items {
		if req.Status == "open" {
			list = append(list, req)
		}
	}
	return list, nil
}

func (r *PasswordResetRepository) Resolve(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.items[id]
	if !ok || req.Status != "open" {
		return fmt.Errorf("%w: password reset %s", domain.ErrNotFound, id)
	}
	now := time.Now().UTC()
	req.Status = "done"
	req.ResolvedAt = &now
	r.items[id] = req
	return nil
}

func (r *PasswordResetRepository) GetByID(ctx context.Context, id uuid.UUID) (userdomain.PasswordResetRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, ok := r.items[id]
	if !ok {
		return userdomain.PasswordResetRequest{}, fmt.Errorf("%w: password reset %s", domain.ErrNotFound, id)
	}
	return req, nil
}
