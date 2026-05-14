package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations 在启动时自动执行未应用的数据库迁移文件
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS) error {
	// 确保迁移记录表存在
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败: %w", err)
	}

	// 读取已应用的迁移
	rows, err := pool.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("查询已应用迁移失败: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		applied[name] = true
	}
	rows.Close()

	// 收集 migrations/ 目录下的 .sql 文件
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("读取 migrations 目录失败: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}

	// init.sql 优先，其余按文件名（日期）升序
	sort.Slice(files, func(i, j int) bool {
		if files[i] == "init.sql" {
			return true
		}
		if files[j] == "init.sql" {
			return false
		}
		return files[i] < files[j]
	})

	for _, name := range files {
		if applied[name] {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
		}

		if err := runMigration(ctx, pool, name, string(content)); err != nil {
			return err
		}

		log.Printf("✅ 迁移已应用: %s\n", name)
	}

	return nil
}

// runMigration 在单个事务内执行迁移 SQL 并记录到 schema_migrations，
// 保证两步原子完成：SQL 失败则记录不写入，不会出现死循环重试。
func runMigration(ctx context.Context, pool *pgxpool.Pool, name, sql string) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("开启事务失败 (%s): %w", name, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES($1)`, name); err != nil {
		return fmt.Errorf("记录迁移 %s 失败: %w", name, err)
	}

	return tx.Commit(ctx)
}
