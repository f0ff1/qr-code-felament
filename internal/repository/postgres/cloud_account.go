package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"filamenttracker/internal/domain"
	cloudaccount "filamenttracker/internal/domain/cloudaccount"

	"github.com/google/uuid"
)

type CloudAccountRepository struct{ db *sql.DB }

func NewCloudAccountRepository(db *sql.DB) *CloudAccountRepository {
	return &CloudAccountRepository{db: db}
}

func (r *CloudAccountRepository) Upsert(ctx context.Context, account cloudaccount.Account) error {
	email := strings.TrimSpace(account.Email)
	var existingID uuid.UUID
	err := r.db.QueryRowContext(ctx, `SELECT id FROM bambu_cloud_accounts WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&existingID)
	if err == nil {
		account.ID = existingID
		_, err = r.db.ExecContext(ctx, `UPDATE bambu_cloud_accounts SET email=$1, password=$2, token=$3, region=$4, updated_at=$5 WHERE id=$6`,
			account.Email, account.Password, account.Token, account.Region, account.UpdatedAt, account.ID)
		return err
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO bambu_cloud_accounts (id, email, password, token, region, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		account.ID, account.Email, account.Password, account.Token, account.Region, account.CreatedAt, account.UpdatedAt)
	return err
}

func (r *CloudAccountRepository) GetLatest(ctx context.Context) (cloudaccount.Account, error) {
	var a cloudaccount.Account
	row := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(email,''), COALESCE(password,''), COALESCE(token,''), COALESCE(region,'us'), created_at, updated_at FROM bambu_cloud_accounts ORDER BY updated_at DESC LIMIT 1`)
	if err := row.Scan(&a.ID, &a.Email, &a.Password, &a.Token, &a.Region, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return cloudaccount.Account{}, fmt.Errorf("%w: cloud account", domain.ErrNotFound)
		}
		return cloudaccount.Account{}, err
	}
	return a, nil
}

func (r *CloudAccountRepository) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bambu_cloud_accounts`)
	return err
}
