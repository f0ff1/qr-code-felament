package cloudaccount

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID        uuid.UUID
	Email     string
	Password  string
	Token     string
	Region    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(email, password, token, region string) Account {
	now := time.Now()
	return Account{
		ID:        uuid.New(),
		Email:     strings.TrimSpace(email),
		Password:  strings.TrimSpace(password),
		Token:     strings.TrimSpace(token),
		Region:    strings.TrimSpace(region),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (a Account) Linked() bool {
	return strings.TrimSpace(a.Token) != ""
}
