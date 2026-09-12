package spool

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, s Spool) error
	GetByID(ctx context.Context, id uuid.UUID) (Spool, error)
	GetByQRToken(ctx context.Context, token string) (Spool, error)
	List(ctx context.Context) ([]Spool, error)
	Update(ctx context.Context, s Spool) error
	Delete(ctx context.Context, id uuid.UUID) error
}
