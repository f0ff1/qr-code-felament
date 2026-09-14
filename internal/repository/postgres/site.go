package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"
	sitedomain "filamenttracker/internal/domain/site"

	"github.com/google/uuid"
)

type SiteRepository struct{ db *sql.DB }

func NewSiteRepository(db *sql.DB) *SiteRepository {
	return &SiteRepository{db: db}
}

func (r *SiteRepository) Create(ctx context.Context, s sitedomain.Site) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sites (id, organization_id, name, address, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.OrganizationID, s.Name, s.Address, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: site create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *SiteRepository) Update(ctx context.Context, s sitedomain.Site) error {
	result, err := r.db.ExecContext(ctx, `UPDATE sites SET organization_id=$1, name=$2, address=$3, updated_at=$4 WHERE id=$5`,
		s.OrganizationID, s.Name, s.Address, s.UpdatedAt, s.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: site %s", domain.ErrNotFound, s.ID)
	}
	return nil
}

func (r *SiteRepository) ListByOrg(ctx context.Context, organizationID uuid.UUID) ([]sitedomain.Site, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, name, address, created_at, updated_at FROM sites WHERE organization_id = $1 ORDER BY created_at ASC`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]sitedomain.Site, 0)
	for rows.Next() {
		var s sitedomain.Site
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.Address, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *SiteRepository) ListAll(ctx context.Context) ([]sitedomain.Site, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, name, address, created_at, updated_at FROM sites ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]sitedomain.Site, 0)
	for rows.Next() {
		var s sitedomain.Site
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.Address, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *SiteRepository) GetByID(ctx context.Context, id uuid.UUID) (sitedomain.Site, error) {
	var s sitedomain.Site
	row := r.db.QueryRowContext(ctx, `SELECT id, organization_id, name, address, created_at, updated_at FROM sites WHERE id = $1`, id)
	if err := row.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.Address, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return sitedomain.Site{}, fmt.Errorf("%w: site %s", domain.ErrNotFound, id)
		}
		return sitedomain.Site{}, err
	}
	return s, nil
}

func (r *SiteRepository) EnsureDefault(ctx context.Context) error {
	_, err := r.GetByID(ctx, org.DefaultSiteID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
INSERT INTO sites (id, organization_id, name, address, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING`,
		org.DefaultSiteID, org.DefaultOrganizationID, "Основной склад", "", now, now)
	return err
}
