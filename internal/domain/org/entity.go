package org

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(name string) Organization {
	now := time.Now().UTC()
	return Organization{
		ID:        uuid.New(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// DefaultIDs match migration 009 bootstrap UUIDs.
var (
	DefaultOrganizationID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	DefaultSiteID         = uuid.MustParse("00000000-0000-4000-8000-000000000002")
)
