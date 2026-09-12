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
	_, err := r.db.ExecContext(ctx, `INSERT INTO printers (id, name, model, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`, p.ID, p.Name, p.Model, string(p.Status), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: printer create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *PrinterRepository) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	var p printerdomain.Printer
	row := r.db.QueryRowContext(ctx, `SELECT id, name, model, status, created_at, updated_at FROM printers WHERE id = $1`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Model, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return printerdomain.Printer{}, fmt.Errorf("%w: printer %s", domain.ErrNotFound, id)
		}
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (r *PrinterRepository) List(ctx context.Context) ([]printerdomain.Printer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, model, status, created_at, updated_at FROM printers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]printerdomain.Printer, 0)
	for rows.Next() {
		var p printerdomain.Printer
		if err := rows.Scan(&p.ID, &p.Name, &p.Model, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *PrinterRepository) Update(ctx context.Context, p printerdomain.Printer) error {
	result, err := r.db.ExecContext(ctx, `UPDATE printers SET name=$1, model=$2, status=$3, updated_at=$4 WHERE id=$5`, p.Name, p.Model, string(p.Status), p.UpdatedAt, p.ID)
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

func (r *PrintJobRepository) Create(ctx context.Context, j printjobdomain.PrintJob) error {
	var finishedAt interface{}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO print_jobs (id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, j.ID, j.PrinterID, j.ProductID, j.SpoolID, string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, j.CreatedAt, j.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: print job create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *PrintJobRepository) GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error) {
	var j printjobdomain.PrintJob
	var finishedAt sql.NullTime
	row := r.db.QueryRowContext(ctx, `SELECT id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, created_at, updated_at FROM print_jobs WHERE id = $1`, id)
	if err := row.Scan(&j.ID, &j.PrinterID, &j.ProductID, &j.SpoolID, &j.Status, &j.Progress, &j.StartedAt, &finishedAt, &j.EstimatedWeight, &j.ConsumedWeight, &j.CreatedAt, &j.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return printjobdomain.PrintJob{}, fmt.Errorf("%w: print job %s", domain.ErrNotFound, id)
		}
		return printjobdomain.PrintJob{}, err
	}
	if finishedAt.Valid {
		j.FinishedAt = &finishedAt.Time
	}
	return j, nil
}

func (r *PrintJobRepository) List(ctx context.Context) ([]printjobdomain.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, printer_id, product_id, spool_id, status, progress, started_at, finished_at, estimated_weight, consumed_weight, created_at, updated_at FROM print_jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]printjobdomain.PrintJob, 0)
	for rows.Next() {
		var j printjobdomain.PrintJob
		var finishedAt sql.NullTime
		if err := rows.Scan(&j.ID, &j.PrinterID, &j.ProductID, &j.SpoolID, &j.Status, &j.Progress, &j.StartedAt, &finishedAt, &j.EstimatedWeight, &j.ConsumedWeight, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			j.FinishedAt = &finishedAt.Time
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
	result, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET printer_id=$1, product_id=$2, spool_id=$3, status=$4, progress=$5, started_at=$6, finished_at=$7, estimated_weight=$8, consumed_weight=$9, updated_at=$10 WHERE id=$11`, j.PrinterID, j.ProductID, j.SpoolID, string(j.Status), j.Progress, j.StartedAt, finishedAt, j.EstimatedWeight, j.ConsumedWeight, j.UpdatedAt, j.ID)
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
