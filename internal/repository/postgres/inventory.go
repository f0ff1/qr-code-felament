package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"filamenttracker/internal/domain"
	invdomain "filamenttracker/internal/domain/inventory"

	"github.com/google/uuid"
)

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
