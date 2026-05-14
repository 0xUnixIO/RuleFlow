package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string
}

type AdminUserRepo struct {
	db *DB
}

func NewAdminUserRepo(db *DB) *AdminUserRepo {
	return &AdminUserRepo{db: db}
}

func (r *AdminUserRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count, err
}

func (r *AdminUserRepo) Create(ctx context.Context, username, passwordHash string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)`,
		username, passwordHash)
	return err
}

func (r *AdminUserRepo) GetByUsername(ctx context.Context, username string) (*AdminUser, error) {
	u := &AdminUser{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, username, password_hash FROM admin_users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}
