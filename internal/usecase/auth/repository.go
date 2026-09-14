package auth

import (
	"context"

	userdomain "filamenttracker/internal/domain/user"

	"github.com/google/uuid"
)

type UserRepository interface {
	Count(ctx context.Context) (int, error)
	GetByUsername(ctx context.Context, username string) (userdomain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (userdomain.User, error)
	Create(ctx context.Context, u userdomain.User) error
	Update(ctx context.Context, u userdomain.User) error
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]userdomain.User, error)
	ListAll(ctx context.Context) ([]userdomain.User, error)
	SetSites(ctx context.Context, userID uuid.UUID, siteIDs []uuid.UUID) error
	GetSiteIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

type PasswordResetRepository interface {
	Create(ctx context.Context, req userdomain.PasswordResetRequest) error
	ListOpen(ctx context.Context) ([]userdomain.PasswordResetRequest, error)
	Resolve(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (userdomain.PasswordResetRequest, error)
}
