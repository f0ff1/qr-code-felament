package product

import (
	"time"

	"github.com/google/uuid"
)

type BillingMode string

const (
	BillingPerson BillingMode = "person"
	BillingLegal  BillingMode = "legal"
	BillingBoth   BillingMode = "both"
)

type Product struct {
	ID                 uuid.UUID
	SiteID             uuid.UUID
	Name               string
	Description        string
	Material           string
	EstimatedWeight    int
	EstimatedPrintTime time.Duration
	Price              float64
	PriceLegal         float64
	BillingMode        BillingMode
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
		BillingMode:        BillingPerson,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
}

func NewAutoProduct(name, description, material string, estimatedWeight int, estimatedPrintTime time.Duration, pricePerson, priceLegal float64) Product {
	p := NewProduct(name, description, material, estimatedWeight, estimatedPrintTime, pricePerson)
	p.PriceLegal = priceLegal
	p.BillingMode = BillingBoth
	return p
}
