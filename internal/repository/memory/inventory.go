package memory

import (
	"context"
	"fmt"

	"filamenttracker/internal/domain"
	invdomain "filamenttracker/internal/domain/inventory"
)

type InventoryRepository struct {
	transactions []invdomain.Transaction
}

func NewInventoryRepository() *InventoryRepository {
	return &InventoryRepository{transactions: make([]invdomain.Transaction, 0)}
}

func (r *InventoryRepository) Add(ctx context.Context, tx invdomain.Transaction) error {
	if tx.Weight == 0 {
		return fmt.Errorf("%w: transaction weight cannot be zero", domain.ErrInvalid)
	}
	r.transactions = append(r.transactions, tx)
	return nil
}

func (r *InventoryRepository) List(ctx context.Context) ([]invdomain.Transaction, error) {
	return r.transactions, nil
}
