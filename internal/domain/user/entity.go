package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

func (r Role) IsAdmin() bool { return r == RoleAdmin }

type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Username       string
	PasswordHash   string
	FirstName      string
	LastName       string
	Role           Role
	IsActive       bool
	SiteIDs        []uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (u User) DisplayName() string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	return u.Username
}

type PasswordResetRequest struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Status     string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}
