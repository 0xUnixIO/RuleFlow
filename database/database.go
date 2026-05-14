package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB 数据库连接池
type DB struct {
	Pool *pgxpool.Pool
}

// New 创建新的数据库连接池
func New(databaseURL string) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接池失败: %w", err)
	}

	// 验证连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close 关闭数据库连接池
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Ping 检查数据库连接状态
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Health 检查数据库健康状态
func (db *DB) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status := map[string]string{}
	if err := db.Ping(ctx); err != nil {
		status["status"] = "unhealthy"
		status["error"] = err.Error()
		return status
	}

	status["status"] = "healthy"
	status["connections"] = fmt.Sprintf("%d/%d",
		db.Pool.Stat().TotalConns(),
		db.Pool.Stat().MaxConns(),
	)

	return status
}
