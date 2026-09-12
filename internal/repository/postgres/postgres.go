package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"filamenttracker/internal/domain"
	cloudaccount "filamenttracker/internal/domain/cloudaccount"
	invdomain "filamenttracker/internal/domain/inventory"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func Open(dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("empty database DSN")
	}
	dsn = normalizeDSN(dsn)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Tuned for ~8 vCPU / 8 GiB app + Postgres: enough concurrency without saturating DB.
	db.SetMaxOpenConns(64)
	db.SetMaxIdleConns(16)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	stats := db.Stats()
	log.Printf("postgres connected host=%s max_open=%d open=%d idle=%d", redactedHost(dsn), stats.MaxOpenConnections, stats.OpenConnections, stats.Idle)
	return db, nil
}

func normalizeDSN(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		return dsn
	}
	query := parsed.Query()
	if query.Get("sslmode") == "" {
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "postgres" {
			query.Set("sslmode", "disable")
		} else {
			query.Set("sslmode", "require")
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn
}

func redactedHost(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host == "" {
		return "(unparsed)"
	}
	return parsed.Host
}

func sqlDebug() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SQL_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}

func Migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS spools (
			id UUID PRIMARY KEY,
			qr_token TEXT NOT NULL UNIQUE,
			material TEXT NOT NULL,
			color TEXT NOT NULL,
			manufacturer TEXT NOT NULL,
			initial_weight INTEGER NOT NULL,
			current_weight INTEGER NOT NULL,
			price DOUBLE PRECISION NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS printers (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			model TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			material TEXT NOT NULL,
			estimated_weight INTEGER NOT NULL,
			estimated_print_time TEXT NOT NULL,
			price DOUBLE PRECISION NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS print_jobs (
			id UUID PRIMARY KEY,
			printer_id UUID NOT NULL,
			product_id UUID NOT NULL,
			spool_id UUID NOT NULL,
			status TEXT NOT NULL,
			progress DOUBLE PRECISION NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ,
			estimated_weight INTEGER NOT NULL,
			consumed_weight INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_transactions (
			id UUID PRIMARY KEY,
			spool_id UUID NOT NULL,
			type TEXT NOT NULL,
			weight INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS lan_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS lan_serial TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS lan_access_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS lan_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS default_spool_id UUID`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS cloud_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS cloud_email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS cloud_password TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS cloud_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE printers ADD COLUMN IF NOT EXISTS cloud_region TEXT NOT NULL DEFAULT 'us'`,
		`ALTER TABLE print_jobs ALTER COLUMN product_id DROP NOT NULL`,
		`ALTER TABLE print_jobs ALTER COLUMN spool_id DROP NOT NULL`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS external_task_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS file_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS is_draft BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS remaining_minutes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS estimated_duration_sec INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS layer_current INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS layer_total INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_print_jobs_external_task ON print_jobs (printer_id, external_task_id)`,
		`CREATE TABLE IF NOT EXISTS bambu_cloud_accounts (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT 'us',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}
	for _, stmt := range stmts {
		if sqlDebug() {
			log.Printf("sql migrate: %s", stmt)
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	log.Printf("postgres migrations applied, pool open=%d idle=%d", db.Stats().OpenConnections, db.Stats().Idle)
	return nil
}

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

type ProductRepository struct{ db *sql.DB }

func NewProductRepository(db *sql.DB) *ProductRepository { return &ProductRepository{db: db} }

func (r *ProductRepository) Create(ctx context.Context, p productdomain.Product) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO products (id,name,description,material,estimated_weight,estimated_print_time,price,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, p.ID, p.Name, p.Description, p.Material, p.EstimatedWeight, p.EstimatedPrintTime.String(), p.Price, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: product create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error) {
	var p productdomain.Product
	var duration string
	row := r.db.QueryRowContext(ctx, `SELECT id, name, description, material, estimated_weight, estimated_print_time, price, created_at, updated_at FROM products WHERE id = $1`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Material, &p.EstimatedWeight, &duration, &p.Price, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return productdomain.Product{}, fmt.Errorf("%w: product %s", domain.ErrNotFound, id)
		}
		return productdomain.Product{}, err
	}
	parsed, err := time.ParseDuration(duration)
	if err != nil {
		return productdomain.Product{}, err
	}
	p.EstimatedPrintTime = parsed
	return p, nil
}

func (r *ProductRepository) List(ctx context.Context) ([]productdomain.Product, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, material, estimated_weight, estimated_print_time, price, created_at, updated_at FROM products ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]productdomain.Product, 0)
	for rows.Next() {
		var p productdomain.Product
		var duration string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Material, &p.EstimatedWeight, &duration, &p.Price, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		parsed, err := time.ParseDuration(duration)
		if err != nil {
			return nil, err
		}
		p.EstimatedPrintTime = parsed
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *ProductRepository) Update(ctx context.Context, p productdomain.Product) error {
	result, err := r.db.ExecContext(ctx, `UPDATE products SET name=$1, description=$2, material=$3, estimated_weight=$4, estimated_print_time=$5, price=$6, updated_at=$7 WHERE id=$8`, p.Name, p.Description, p.Material, p.EstimatedWeight, p.EstimatedPrintTime.String(), p.Price, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: product %s", domain.ErrNotFound, p.ID)
	}
	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: product %s", domain.ErrNotFound, id)
	}
	return nil
}

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
	var finishedAt interface{}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	source := string(j.Source)
	if source == "" {
		source = string(printjobdomain.SourceManual)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO print_jobs (id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, source, external_task_id, file_name, is_draft, remaining_minutes, estimated_duration_sec, layer_current, layer_total, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		j.ID, j.PrinterID, nullableUUID(j.ProductID), nullableUUID(j.SpoolID), string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, source, j.ExternalTaskID, j.FileName, j.IsDraft, j.RemainingMinutes, j.EstimatedDurationSec, j.LayerCurrent, j.LayerTotal, j.CreatedAt, j.UpdatedAt)
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
	if err := scan(&j.ID, &j.PrinterID, &productID, &spoolID, &j.Status, &j.Progress, &j.StartedAt, &finishedAt, &j.EstimatedWeight, &j.ConsumedWeight, &source, &externalTaskID, &fileName, &isDraft, &j.RemainingMinutes, &j.EstimatedDurationSec, &j.LayerCurrent, &j.LayerTotal, &j.CreatedAt, &j.UpdatedAt); err != nil {
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

const printJobSelect = `SELECT id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, COALESCE(source,'manual'), COALESCE(external_task_id,''), COALESCE(file_name,''), COALESCE(is_draft,false), COALESCE(remaining_minutes,0), COALESCE(estimated_duration_sec,0), COALESCE(layer_current,0), COALESCE(layer_total,0), created_at, updated_at FROM print_jobs`

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
	var finishedAt interface{}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	source := string(j.Source)
	if source == "" {
		source = string(printjobdomain.SourceManual)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET printer_id=$1, product_id=$2, spool_id=$3, status=$4, progress=$5, started_at=$6, finished_at=$7, estimated_weight=$8, consumed_weight=$9, source=$10, external_task_id=$11, file_name=$12, is_draft=$13, remaining_minutes=$14, estimated_duration_sec=$15, layer_current=$16, layer_total=$17, updated_at=$18 WHERE id=$19`,
		j.PrinterID, nullableUUID(j.ProductID), nullableUUID(j.SpoolID), string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, source, j.ExternalTaskID, j.FileName, j.IsDraft, j.RemainingMinutes, j.EstimatedDurationSec, j.LayerCurrent, j.LayerTotal, j.UpdatedAt, j.ID)
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

type InventoryRepository struct{ db *sql.DB }

func NewInventoryRepository(db *sql.DB) *InventoryRepository { return &InventoryRepository{db: db} }

func (r *InventoryRepository) Add(ctx context.Context, tx invdomain.Transaction) error {
	id, err := uuid.Parse(tx.ID)
	if err != nil {
		return fmt.Errorf("%w: invalid transaction id: %v", domain.ErrInvalid, err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO inventory_transactions (id, spool_id, type, weight, created_at) VALUES ($1,$2,$3,$4,$5)`, id, tx.SpoolID, tx.Type, tx.Weight, tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("%w: inventory add: %v", domain.ErrInvalid, err)
	}
	return nil
}

func (r *InventoryRepository) List(ctx context.Context) ([]invdomain.Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, spool_id, type, weight, created_at FROM inventory_transactions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]invdomain.Transaction, 0)
	for rows.Next() {
		var tx invdomain.Transaction
		if err := rows.Scan(&tx.ID, &tx.SpoolID, &tx.Type, &tx.Weight, &tx.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, tx)
	}
	return items, rows.Err()
}

type CloudAccountRepository struct{ db *sql.DB }

func NewCloudAccountRepository(db *sql.DB) *CloudAccountRepository {
	return &CloudAccountRepository{db: db}
}

func (r *CloudAccountRepository) Upsert(ctx context.Context, account cloudaccount.Account) error {
	email := strings.TrimSpace(account.Email)
	var existingID uuid.UUID
	err := r.db.QueryRowContext(ctx, `SELECT id FROM bambu_cloud_accounts WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&existingID)
	if err == nil {
		account.ID = existingID
		_, err = r.db.ExecContext(ctx, `UPDATE bambu_cloud_accounts SET email=$1, password=$2, token=$3, region=$4, updated_at=$5 WHERE id=$6`,
			account.Email, account.Password, account.Token, account.Region, account.UpdatedAt, account.ID)
		return err
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO bambu_cloud_accounts (id, email, password, token, region, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		account.ID, account.Email, account.Password, account.Token, account.Region, account.CreatedAt, account.UpdatedAt)
	return err
}

func (r *CloudAccountRepository) GetLatest(ctx context.Context) (cloudaccount.Account, error) {
	var a cloudaccount.Account
	row := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(email,''), COALESCE(password,''), COALESCE(token,''), COALESCE(region,'us'), created_at, updated_at FROM bambu_cloud_accounts ORDER BY updated_at DESC LIMIT 1`)
	if err := row.Scan(&a.ID, &a.Email, &a.Password, &a.Token, &a.Region, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return cloudaccount.Account{}, fmt.Errorf("%w: cloud account", domain.ErrNotFound)
		}
		return cloudaccount.Account{}, err
	}
	return a, nil
}

func (r *CloudAccountRepository) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bambu_cloud_accounts`)
	return err
}
