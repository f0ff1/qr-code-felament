package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"filamenttracker/internal/domain"
	userdomain "filamenttracker/internal/domain/user"

	"github.com/google/uuid"
)

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

type PasswordResetRepository struct{ db *sql.DB }

func NewPasswordResetRepository(db *sql.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u userdomain.User) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO users (id, organization_id, username, password_hash, first_name, last_name, role, is_active, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.ID, u.OrganizationID, u.Username, u.PasswordHash, u.FirstName, u.LastName, string(u.Role), u.IsActive, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%w: user create: %v", domain.ErrConflict, err)
	}
	if len(u.SiteIDs) > 0 {
		if err := r.SetSites(ctx, u.ID, u.SiteIDs); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u userdomain.User) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET organization_id=$1, username=$2, password_hash=$3, first_name=$4, last_name=$5, role=$6, is_active=$7, updated_at=$8 WHERE id=$9`,
		u.OrganizationID, u.Username, u.PasswordHash, u.FirstName, u.LastName, string(u.Role), u.IsActive, u.UpdatedAt, u.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: user %s", domain.ErrNotFound, u.ID)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (userdomain.User, error) {
	var u userdomain.User
	var role string
	row := r.db.QueryRowContext(ctx, `SELECT id, organization_id, username, password_hash, first_name, last_name, role, is_active, created_at, updated_at FROM users WHERE id = $1`, id)
	if err := row.Scan(&u.ID, &u.OrganizationID, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return userdomain.User{}, fmt.Errorf("%w: user %s", domain.ErrNotFound, id)
		}
		return userdomain.User{}, err
	}
	u.Role = userdomain.Role(role)
	sites, err := r.GetSiteIDs(ctx, u.ID)
	if err != nil {
		return userdomain.User{}, err
	}
	u.SiteIDs = sites
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (userdomain.User, error) {
	var u userdomain.User
	var role string
	row := r.db.QueryRowContext(ctx, `SELECT id, organization_id, username, password_hash, first_name, last_name, role, is_active, created_at, updated_at FROM users WHERE lower(username) = lower($1)`, username)
	if err := row.Scan(&u.ID, &u.OrganizationID, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return userdomain.User{}, fmt.Errorf("%w: user %s", domain.ErrNotFound, username)
		}
		return userdomain.User{}, err
	}
	u.Role = userdomain.Role(role)
	sites, err := r.GetSiteIDs(ctx, u.ID)
	if err != nil {
		return userdomain.User{}, err
	}
	u.SiteIDs = sites
	return u, nil
}

func (r *UserRepository) ListByOrg(ctx context.Context, organizationID uuid.UUID) ([]userdomain.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, username, password_hash, first_name, last_name, role, is_active, created_at, updated_at FROM users WHERE organization_id = $1 ORDER BY created_at ASC`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]userdomain.User, 0)
	for rows.Next() {
		var u userdomain.User
		var role string
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.Username, &u.PasswordHash, &u.FirstName, &u.LastName, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Role = userdomain.Role(role)
		sites, err := r.GetSiteIDs(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		u.SiteIDs = sites
		items = append(items, u)
	}
	return items, rows.Err()
}

func (r *UserRepository) SetSites(ctx context.Context, userID uuid.UUID, siteIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_sites WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, siteID := range siteIDs {
		if siteID == uuid.Nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_sites (user_id, site_id) VALUES ($1, $2)`, userID, siteID); err != nil {
			return fmt.Errorf("%w: user site: %v", domain.ErrConflict, err)
		}
	}
	return tx.Commit()
}

func (r *UserRepository) GetSiteIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT site_id FROM user_sites WHERE user_id = $1 ORDER BY site_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *PasswordResetRepository) Create(ctx context.Context, req userdomain.PasswordResetRequest) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO password_reset_requests (id, user_id, status, created_at, resolved_at) VALUES ($1,$2,$3,$4,$5)`,
		req.ID, req.UserID, req.Status, req.CreatedAt, req.ResolvedAt)
	if err != nil {
		return fmt.Errorf("%w: password reset create: %v", domain.ErrConflict, err)
	}
	return nil
}

func (r *PasswordResetRepository) ListOpen(ctx context.Context) ([]userdomain.PasswordResetRequest, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, status, created_at, resolved_at FROM password_reset_requests WHERE status = 'open' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]userdomain.PasswordResetRequest, 0)
	for rows.Next() {
		var req userdomain.PasswordResetRequest
		var resolvedAt sql.NullTime
		if err := rows.Scan(&req.ID, &req.UserID, &req.Status, &req.CreatedAt, &resolvedAt); err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			t := resolvedAt.Time
			req.ResolvedAt = &t
		}
		items = append(items, req)
	}
	return items, rows.Err()
}

func (r *PasswordResetRepository) Resolve(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `UPDATE password_reset_requests SET status = 'done', resolved_at = $1 WHERE id = $2 AND status = 'open'`, now, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: password reset %s", domain.ErrNotFound, id)
	}
	return nil
}

func (r *PasswordResetRepository) GetByID(ctx context.Context, id uuid.UUID) (userdomain.PasswordResetRequest, error) {
	var req userdomain.PasswordResetRequest
	var resolvedAt sql.NullTime
	row := r.db.QueryRowContext(ctx, `SELECT id, user_id, status, created_at, resolved_at FROM password_reset_requests WHERE id = $1`, id)
	if err := row.Scan(&req.ID, &req.UserID, &req.Status, &req.CreatedAt, &resolvedAt); err != nil {
		if err == sql.ErrNoRows {
			return userdomain.PasswordResetRequest{}, fmt.Errorf("%w: password reset %s", domain.ErrNotFound, id)
		}
		return userdomain.PasswordResetRequest{}, err
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		req.ResolvedAt = &t
	}
	return req, nil
}
