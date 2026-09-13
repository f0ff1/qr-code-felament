package site

import (
	"context"

	sitedomain "filamenttracker/internal/domain/site"

	"github.com/google/uuid"
)

type SiteRepository interface {
	Create(ctx context.Context, s sitedomain.Site) error
	Update(ctx context.Context, s sitedomain.Site) error
	ListByOrg(ctx context.Context, organizationID uuid.UUID) ([]sitedomain.Site, error)
	GetByID(ctx context.Context, id uuid.UUID) (sitedomain.Site, error)
	EnsureDefault(ctx context.Context) error
}
