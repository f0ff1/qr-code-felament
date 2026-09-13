package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"

	"github.com/google/uuid"
)

type OrganizationRepository struct{ db *sql.DB }

func NewOrganizationRepository(db *sql.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Upsert(ctx context.Context, o org.Organization) error {
	now := time.Now().UTC()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	o.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, `
INSERT INTO organizations (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = EXCLUDED.updated_at`,
		o.ID, o.Name, o.CreatedAt, o.UpdatedAt)
	return err
}

func (r *OrganizationRepository) Update(ctx context.Context, o org.Organization) error {
	result, err := r.db.ExecContext(ctx, `UPDATE organizations SET name=$1, updated_at=$2 WHERE id=$3`, o.Name, o.UpdatedAt, o.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: organization %s", domain.ErrNotFound, o.ID)
	}
	return nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (org.Organization, error) {
	var o org.Organization
	row := r.db.QueryRowContext(ctx, `SELECT id, name, created_at, updated_at FROM organizations WHERE id = $1`, id)
	if err := row.Scan(&o.ID, &o.Name, &o.CreatedAt, &o.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return org.Organization{}, fmt.Errorf("%w: organization %s", domain.ErrNotFound, id)
		}
		return org.Organization{}, err
	}
	return o, nil
}

func (r *OrganizationRepository) GetDefault(ctx context.Context) (org.Organization, error) {
	return r.GetByID(ctx, org.DefaultOrganizationID)
}

func (r *OrganizationRepository) EnsureDefault(ctx context.Context) error {
	_, err := r.GetByID(ctx, org.DefaultOrganizationID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO organizations (id, name, created_at, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING`,
		org.DefaultOrganizationID, "Default Company", now, now)
	return err
}
