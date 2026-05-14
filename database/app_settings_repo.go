package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
)

// LoadOrCreateSessionSecret 从 app_settings 读取 session_secret，
// 不存在时自动生成并持久化，保证每次启动使用同一密钥。
func LoadOrCreateSessionSecret(ctx context.Context, db *DB) (string, error) {
	var secret string
	err := db.Pool.QueryRow(ctx,
		`SELECT value FROM app_settings WHERE key = 'session_secret'`,
	).Scan(&secret)

	if err == nil {
		return secret, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	secret = hex.EncodeToString(b)

	_, err = db.Pool.Exec(ctx,
		`INSERT INTO app_settings (key, value) VALUES ('session_secret', $1) ON CONFLICT DO NOTHING`,
		secret,
	)
	return secret, err
}
