package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	productdomain "filamenttracker/internal/domain/product"

	"github.com/google/uuid"
)

type ProductRepository struct{ db *sql.DB }

func NewProductRepository(db *sql.DB) *ProductRepository { return &ProductRepository{db: db} }

func (r *ProductRepository) Create(ctx context.Context, p productdomain.Product) error {
	mode := string(p.BillingMode)
	if mode == "" {
		mode = string(productdomain.BillingPerson)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO products (id,name,description,material,estimated_weight,estimated_print_time,price,price_legal,billing_mode,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, p.ID, p.Name, p.Description, p.Material, p.EstimatedWeight, p.EstimatedPrintTime.String(), p.Price, p.PriceLegal, mode, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: product create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error) {
	var p productdomain.Product
	var duration string
	var mode string
	row := r.db.QueryRowContext(ctx, `SELECT id, name, description, material, estimated_weight, estimated_print_time, price, COALESCE(price_legal,0), COALESCE(billing_mode,'person'), created_at, updated_at FROM products WHERE id = $1`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Material, &p.EstimatedWeight, &duration, &p.Price, &p.PriceLegal, &mode, &p.CreatedAt, &p.UpdatedAt); err != nil {
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
	p.BillingMode = productdomain.BillingMode(mode)
	return p, nil
}

func (r *ProductRepository) List(ctx context.Context) ([]productdomain.Product, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, material, estimated_weight, estimated_print_time, price, COALESCE(price_legal,0), COALESCE(billing_mode,'person'), created_at, updated_at FROM products ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]productdomain.Product, 0)
	for rows.Next() {
		var p productdomain.Product
		var duration string
		var mode string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Material, &p.EstimatedWeight, &duration, &p.Price, &p.PriceLegal, &mode, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		parsed, err := time.ParseDuration(duration)
		if err != nil {
			return nil, err
		}
		p.EstimatedPrintTime = parsed
		p.BillingMode = productdomain.BillingMode(mode)
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *ProductRepository) Update(ctx context.Context, p productdomain.Product) error {
	mode := string(p.BillingMode)
	if mode == "" {
		mode = string(productdomain.BillingPerson)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE products SET name=$1, description=$2, material=$3, estimated_weight=$4, estimated_print_time=$5, price=$6, price_legal=$7, billing_mode=$8, updated_at=$9 WHERE id=$10`, p.Name, p.Description, p.Material, p.EstimatedWeight, p.EstimatedPrintTime.String(), p.Price, p.PriceLegal, mode, p.UpdatedAt, p.ID)
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
