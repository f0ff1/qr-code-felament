package product

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID                 uuid.UUID
	Name               string
	Description        string
	Material           string
	EstimatedWeight    int
	EstimatedPrintTime time.Duration
	Price              float64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewProduct(name, description, material string, estimatedWeight int, estimatedPrintTime time.Duration, price float64) Product {
	return Product{
		ID:                 uuid.New(),
		Name:               name,
		Description:        description,
		Material:           material,
		EstimatedWeight:    estimatedWeight,
		EstimatedPrintTime: estimatedPrintTime,
		Price:              price,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
}
