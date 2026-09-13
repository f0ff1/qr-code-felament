package org

import (
	"context"

	"filamenttracker/internal/domain/org"

	"github.com/google/uuid"
)

type OrganizationRepository interface {
	Upsert(ctx context.Context, o org.Organization) error
	Update(ctx context.Context, o org.Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (org.Organization, error)
	GetDefault(ctx context.Context) (org.Organization, error)
	EnsureDefault(ctx context.Context) error
}
