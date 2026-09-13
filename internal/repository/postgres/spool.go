package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/org"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type SpoolRepository struct{ db *sql.DB }

func NewSpoolRepository(db *sql.DB) *SpoolRepository { return &SpoolRepository{db: db} }

func siteOrDefault(id uuid.UUID) uuid.UUID {
	if id == uuid.Nil {
		return org.DefaultSiteID
	}
	return id
}

func (r *SpoolRepository) Create(ctx context.Context, s spooldomain.Spool) error {
	s.SiteID = siteOrDefault(s.SiteID)
	_, err := r.db.ExecContext(ctx, `INSERT INTO spools (id, site_id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, s.ID, s.SiteID, s.QRToken, string(s.Material), s.Color, s.Manufacturer, s.InitialWeight, s.CurrentWeight, s.Price, string(s.Status), s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: spool create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *SpoolRepository) GetByID(ctx context.Context, id uuid.UUID) (spooldomain.Spool, error) {
	var s spooldomain.Spool
	row := r.db.QueryRowContext(ctx, `SELECT id, site_id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools WHERE id = $1`, id)
	if err := row.Scan(&s.ID, &s.SiteID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return spooldomain.Spool{}, fmt.Errorf("%w: spool %s", domain.ErrNotFound, id)
		}
		return spooldomain.Spool{}, err
	}
	return s, nil
}

func (r *SpoolRepository) GetByQRToken(ctx context.Context, token string) (spooldomain.Spool, error) {
	var s spooldomain.Spool
	row := r.db.QueryRowContext(ctx, `SELECT id, site_id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools WHERE qr_token = $1`, token)
	if err := row.Scan(&s.ID, &s.SiteID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return spooldomain.Spool{}, fmt.Errorf("%w: spool token %s", domain.ErrNotFound, token)
		}
		return spooldomain.Spool{}, err
	}
	return s, nil
}

func (r *SpoolRepository) List(ctx context.Context) ([]spooldomain.Spool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, site_id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]spooldomain.Spool, 0)
	for rows.Next() {
		var s spooldomain.Spool
		if err := rows.Scan(&s.ID, &s.SiteID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *SpoolRepository) Update(ctx context.Context, s spooldomain.Spool) error {
	s.SiteID = siteOrDefault(s.SiteID)
	result, err := r.db.ExecContext(ctx, `UPDATE spools SET site_id=$1, qr_token=$2, material=$3, color=$4, manufacturer=$5, initial_weight=$6, current_weight=$7, price=$8, status=$9, updated_at=$10 WHERE id=$11`, s.SiteID, s.QRToken, string(s.Material), s.Color, s.Manufacturer, s.InitialWeight, s.CurrentWeight, s.Price, string(s.Status), s.UpdatedAt, s.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: spool %s", domain.ErrNotFound, s.ID)
	}
	return nil
}

func (r *SpoolRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM spools WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: spool %s", domain.ErrNotFound, id)
	}
	return nil
}
