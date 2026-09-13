package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"filamenttracker/internal/domain"
	printerdomain "filamenttracker/internal/domain/printer"

	"github.com/google/uuid"
)

type PrinterRepository struct{ db *sql.DB }

func NewPrinterRepository(db *sql.DB) *PrinterRepository { return &PrinterRepository{db: db} }

func (r *PrinterRepository) Create(ctx context.Context, p printerdomain.Printer) error {
	var defaultSpool any
	if p.DefaultSpoolID != uuid.Nil {
		defaultSpool = p.DefaultSpoolID
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO printers (id, name, model, status, lan_host, lan_serial, lan_access_code, lan_enabled, cloud_enabled, cloud_email, cloud_password, cloud_token, cloud_region, default_spool_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		p.ID, p.Name, p.Model, string(p.Status), p.LANHost, p.LANSerial, p.LANAccessCode, p.LANEnabled, p.CloudEnabled, p.CloudEmail, p.CloudPassword, p.CloudToken, p.CloudRegion, defaultSpool, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: printer create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *PrinterRepository) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	var p printerdomain.Printer
	var defaultSpool sql.NullString
	row := r.db.QueryRowContext(ctx, `SELECT id, name, model, status, COALESCE(lan_host,''), COALESCE(lan_serial,''), COALESCE(lan_access_code,''), COALESCE(lan_enabled,false), COALESCE(cloud_enabled,false), COALESCE(cloud_email,''), COALESCE(cloud_password,''), COALESCE(cloud_token,''), COALESCE(cloud_region,'us'), default_spool_id, created_at, updated_at FROM printers WHERE id = $1`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Model, &p.Status, &p.LANHost, &p.LANSerial, &p.LANAccessCode, &p.LANEnabled, &p.CloudEnabled, &p.CloudEmail, &p.CloudPassword, &p.CloudToken, &p.CloudRegion, &defaultSpool, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return printerdomain.Printer{}, fmt.Errorf("%w: printer %s", domain.ErrNotFound, id)
		}
		return printerdomain.Printer{}, err
	}
	if defaultSpool.Valid {
		if parsed, err := uuid.Parse(defaultSpool.String); err == nil {
			p.DefaultSpoolID = parsed
		}
	}
	return p, nil
}

func (r *PrinterRepository) List(ctx context.Context) ([]printerdomain.Printer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, model, status, COALESCE(lan_host,''), COALESCE(lan_serial,''), COALESCE(lan_access_code,''), COALESCE(lan_enabled,false), COALESCE(cloud_enabled,false), COALESCE(cloud_email,''), COALESCE(cloud_password,''), COALESCE(cloud_token,''), COALESCE(cloud_region,'us'), default_spool_id, created_at, updated_at FROM printers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]printerdomain.Printer, 0)
	for rows.Next() {
		var p printerdomain.Printer
		var defaultSpool sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Model, &p.Status, &p.LANHost, &p.LANSerial, &p.LANAccessCode, &p.LANEnabled, &p.CloudEnabled, &p.CloudEmail, &p.CloudPassword, &p.CloudToken, &p.CloudRegion, &defaultSpool, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if defaultSpool.Valid {
			if parsed, err := uuid.Parse(defaultSpool.String); err == nil {
				p.DefaultSpoolID = parsed
			}
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *PrinterRepository) Update(ctx context.Context, p printerdomain.Printer) error {
	var defaultSpool any
	if p.DefaultSpoolID != uuid.Nil {
		defaultSpool = p.DefaultSpoolID
	}
	result, err := r.db.ExecContext(ctx, `UPDATE printers SET name=$1, model=$2, status=$3, lan_host=$4, lan_serial=$5, lan_access_code=$6, lan_enabled=$7, cloud_enabled=$8, cloud_email=$9, cloud_password=$10, cloud_token=$11, cloud_region=$12, default_spool_id=$13, updated_at=$14 WHERE id=$15`,
		p.Name, p.Model, string(p.Status), p.LANHost, p.LANSerial, p.LANAccessCode, p.LANEnabled, p.CloudEnabled, p.CloudEmail, p.CloudPassword, p.CloudToken, p.CloudRegion, defaultSpool, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: printer %s", domain.ErrNotFound, p.ID)
	}
	return nil
}

func (r *PrinterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM printers WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: printer %s", domain.ErrNotFound, id)
	}
	return nil
}
