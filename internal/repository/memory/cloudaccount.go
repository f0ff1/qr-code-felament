package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"filamenttracker/internal/domain"
	cloudaccount "filamenttracker/internal/domain/cloudaccount"
)

type CloudAccountRepository struct {
	mu    sync.Mutex
	items map[string]cloudaccount.Account
}

func NewCloudAccountRepository() *CloudAccountRepository {
	return &CloudAccountRepository{items: make(map[string]cloudaccount.Account)}
}

func (r *CloudAccountRepository) Upsert(ctx context.Context, account cloudaccount.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(account.Email))
	if key == "" {
		key = account.ID.String()
	}
	r.items[key] = account
	return nil
}

func (r *CloudAccountRepository) GetLatest(ctx context.Context) (cloudaccount.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest cloudaccount.Account
	found := false
	for _, a := range r.items {
		if !found || a.UpdatedAt.After(latest.UpdatedAt) {
			latest = a
			found = true
		}
	}
	if !found {
		return cloudaccount.Account{}, fmt.Errorf("%w: cloud account", domain.ErrNotFound)
	}
	return latest, nil
}

func (r *CloudAccountRepository) DeleteAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[string]cloudaccount.Account)
	return nil
}
