package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"filamenttracker/internal/domain"
	printjobdomain "filamenttracker/internal/domain/printjob"

	"github.com/google/uuid"
)

type PrintJobRepository struct{ db *sql.DB }

func NewPrintJobRepository(db *sql.DB) *PrintJobRepository { return &PrintJobRepository{db: db} }

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func scanOptionalUUID(raw sql.NullString) uuid.UUID {
	if !raw.Valid || raw.String == "" {
		return uuid.Nil
	}
	parsed, err := uuid.Parse(raw.String)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func (r *PrintJobRepository) Create(ctx context.Context, j printjobdomain.PrintJob) error {
	j.SiteID = siteOrDefault(j.SiteID)
	var finishedAt interface{}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	source := string(j.Source)
	if source == "" {
		source = string(printjobdomain.SourceManual)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO print_jobs (id, site_id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, source, external_task_id, file_name, is_draft, remaining_minutes, estimated_duration_sec, layer_current, layer_total, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		j.ID, j.SiteID, j.PrinterID, nullableUUID(j.ProductID), nullableUUID(j.SpoolID), string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, source, j.ExternalTaskID, j.FileName, j.IsDraft, j.RemainingMinutes, j.EstimatedDurationSec, j.LayerCurrent, j.LayerTotal, j.CreatedAt, j.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: print job create: %v", domain.ErrConflict, err)
	}
	return nil
}

func scanPrintJob(scan func(dest ...any) error) (printjobdomain.PrintJob, error) {
	var j printjobdomain.PrintJob
	var finishedAt sql.NullTime
	var productID, spoolID sql.NullString
	var source, externalTaskID, fileName string
	var isDraft bool
	if err := scan(&j.ID, &j.SiteID, &j.PrinterID, &productID, &spoolID, &j.Status, &j.Progress, &j.StartedAt, &finishedAt, &j.EstimatedWeight, &j.ConsumedWeight, &source, &externalTaskID, &fileName, &isDraft, &j.RemainingMinutes, &j.EstimatedDurationSec, &j.LayerCurrent, &j.LayerTotal, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return printjobdomain.PrintJob{}, err
	}
	j.ProductID = scanOptionalUUID(productID)
	j.SpoolID = scanOptionalUUID(spoolID)
	if source == "" {
		source = string(printjobdomain.SourceManual)
	}
	j.Source = printjobdomain.Source(source)
	j.ExternalTaskID = externalTaskID
	j.FileName = fileName
	j.IsDraft = isDraft
	if finishedAt.Valid {
		j.FinishedAt = &finishedAt.Time
	}
	return j, nil
}

const printJobSelect = `SELECT id, site_id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, COALESCE(source,'manual'), COALESCE(external_task_id,''), COALESCE(file_name,''), COALESCE(is_draft,false), COALESCE(remaining_minutes,0), COALESCE(estimated_duration_sec,0), COALESCE(layer_current,0), COALESCE(layer_total,0), created_at, updated_at FROM print_jobs`

func (r *PrintJobRepository) GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error) {
	row := r.db.QueryRowContext(ctx, printJobSelect+` WHERE id = $1`, id)
	j, err := scanPrintJob(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return printjobdomain.PrintJob{}, fmt.Errorf("%w: print job %s", domain.ErrNotFound, id)
		}
		return printjobdomain.PrintJob{}, err
	}
	return j, nil
}

func (r *PrintJobRepository) GetByExternalTaskID(ctx context.Context, printerID uuid.UUID, externalTaskID string) (printjobdomain.PrintJob, error) {
	row := r.db.QueryRowContext(ctx, printJobSelect+` WHERE printer_id = $1 AND external_task_id = $2 ORDER BY created_at DESC LIMIT 1`, printerID, externalTaskID)
	j, err := scanPrintJob(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return printjobdomain.PrintJob{}, fmt.Errorf("%w: print job external %s", domain.ErrNotFound, externalTaskID)
		}
		return printjobdomain.PrintJob{}, err
	}
	return j, nil
}

func (r *PrintJobRepository) List(ctx context.Context) ([]printjobdomain.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, printJobSelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]printjobdomain.PrintJob, 0)
	for rows.Next() {
		j, err := scanPrintJob(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, rows.Err()
}

func (r *PrintJobRepository) Update(ctx context.Context, j printjobdomain.PrintJob) error {
	j.SiteID = siteOrDefault(j.SiteID)
	var finishedAt interface{}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	source := string(j.Source)
	if source == "" {
		source = string(printjobdomain.SourceManual)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET site_id=$1, printer_id=$2, product_id=$3, spool_id=$4, status=$5, progress=$6, started_at=$7, finished_at=$8, estimated_weight=$9, consumed_weight=$10, source=$11, external_task_id=$12, file_name=$13, is_draft=$14, remaining_minutes=$15, estimated_duration_sec=$16, layer_current=$17, layer_total=$18, updated_at=$19 WHERE id=$20`,
		j.SiteID, j.PrinterID, nullableUUID(j.ProductID), nullableUUID(j.SpoolID), string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, source, j.ExternalTaskID, j.FileName, j.IsDraft, j.RemainingMinutes, j.EstimatedDurationSec, j.LayerCurrent, j.LayerTotal, j.UpdatedAt, j.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: print job %s", domain.ErrNotFound, j.ID)
	}
	return nil
}

func (r *PrintJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM print_jobs WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: print job %s", domain.ErrNotFound, id)
	}
	return nil
}
