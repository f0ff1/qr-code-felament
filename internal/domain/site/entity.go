package site

import (
	"time"

	"github.com/google/uuid"
)

type Site struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Address        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(organizationID uuid.UUID, name, address string) Site {
	now := time.Now().UTC()
	return Site{
		ID:             uuid.New(),
		OrganizationID: organizationID,
		Name:           name,
		Address:        address,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
