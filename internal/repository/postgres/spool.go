package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"filamenttracker/internal/domain"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type SpoolRepository struct{ db *sql.DB }

func NewSpoolRepository(db *sql.DB) *SpoolRepository { return &SpoolRepository{db: db} }

func (r *SpoolRepository) Create(ctx context.Context, s spooldomain.Spool) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO spools (id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, s.ID, s.QRToken, string(s.Material), s.Color, s.Manufacturer, s.InitialWeight, s.CurrentWeight, s.Price, string(s.Status), s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: spool create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *SpoolRepository) GetByID(ctx context.Context, id uuid.UUID) (spooldomain.Spool, error) {
	var s spooldomain.Spool
	row := r.db.QueryRowContext(ctx, `SELECT id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools WHERE id = $1`, id)
	if err := row.Scan(&s.ID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return spooldomain.Spool{}, fmt.Errorf("%w: spool %s", domain.ErrNotFound, id)
		}
		return spooldomain.Spool{}, err
	}
	return s, nil
}

func (r *SpoolRepository) GetByQRToken(ctx context.Context, token string) (spooldomain.Spool, error) {
	var s spooldomain.Spool
	row := r.db.QueryRowContext(ctx, `SELECT id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools WHERE qr_token = $1`, token)
	if err := row.Scan(&s.ID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return spooldomain.Spool{}, fmt.Errorf("%w: spool token %s", domain.ErrNotFound, token)
		}
		return spooldomain.Spool{}, err
	}
	return s, nil
}

func (r *SpoolRepository) List(ctx context.Context) ([]spooldomain.Spool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, qr_token, material, color, manufacturer, initial_weight, current_weight, price, status, created_at, updated_at FROM spools ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]spooldomain.Spool, 0)
	for rows.Next() {
		var s spooldomain.Spool
		if err := rows.Scan(&s.ID, &s.QRToken, &s.Material, &s.Color, &s.Manufacturer, &s.InitialWeight, &s.CurrentWeight, &s.Price, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *SpoolRepository) Update(ctx context.Context, s spooldomain.Spool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE spools SET qr_token=$1, material=$2, color=$3, manufacturer=$4, initial_weight=$5, current_weight=$6, price=$7, status=$8, updated_at=$9 WHERE id=$10`, s.QRToken, string(s.Material), s.Color, s.Manufacturer, s.InitialWeight, s.CurrentWeight, s.Price, string(s.Status), s.UpdatedAt, s.ID)
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
