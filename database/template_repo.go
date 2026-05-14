package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Template 模板模型
type Template struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Target      string    `json:"target"`
	Tags        []string  `json:"tags"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TemplateRepo 模板仓储
type TemplateRepo struct {
	db *DB
}

// NewTemplateRepo 创建模板仓储
func NewTemplateRepo(db *DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

// Create 创建模板
func (r *TemplateRepo) Create(ctx context.Context, tpl *Template) error {
	tpl.ID = NextID()
	query := `
		INSERT INTO templates (id, name, description, content, target, tags, is_public)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		tpl.ID, tpl.Name, tpl.Description, tpl.Content, tpl.Target, tpl.Tags, tpl.IsPublic,
	).Scan(&tpl.CreatedAt, &tpl.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}

	return nil
}

// GetByName 根据名称获取模板
func (r *TemplateRepo) GetByName(ctx context.Context, name string) (*Template, error) {
	query := `
		SELECT id, name, description, content, target, tags, is_public,
		       created_at, updated_at
		FROM templates
		WHERE name = $1
	`

	tpl := &Template{}
	err := r.db.Pool.QueryRow(ctx, query, name).Scan(
		&tpl.ID,
		&tpl.Name,
		&tpl.Description,
		&tpl.Content,
		&tpl.Target,
		&tpl.Tags,
		&tpl.IsPublic,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("模板不存在: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}

	return tpl, nil
}

// GetByID 根据 ID 获取模板
func (r *TemplateRepo) GetByID(ctx context.Context, id int64) (*Template, error) {
	query := `
		SELECT id, name, description, content, target, tags, is_public,
		       created_at, updated_at
		FROM templates
		WHERE id = $1
	`

	tpl := &Template{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&tpl.ID,
		&tpl.Name,
		&tpl.Description,
		&tpl.Content,
		&tpl.Target,
		&tpl.Tags,
		&tpl.IsPublic,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("模板不存在: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}

	return tpl, nil
}

// List 列出所有模板
func (r *TemplateRepo) List(ctx context.Context) ([]Template, error) {
	query := `
		SELECT id, name, description, content, target, tags, is_public,
		       created_at, updated_at
		FROM templates
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询模板列表失败: %w", err)
	}
	defer rows.Close()

	tpls := []Template{}
	for rows.Next() {
		tpl := Template{}
		err := rows.Scan(
			&tpl.ID,
			&tpl.Name,
			&tpl.Description,
			&tpl.Content,
			&tpl.Target,
			&tpl.Tags,
			&tpl.IsPublic,
			&tpl.CreatedAt,
			&tpl.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描模板行失败: %w", err)
		}
		tpls = append(tpls, tpl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历模板行失败: %w", err)
	}

	return tpls, nil
}

// GetPublicByID 获取公开模板（含内容，仅限 is_public = true）
func (r *TemplateRepo) GetPublicByID(ctx context.Context, id int64) (*Template, error) {
	query := `
		SELECT id, name, description, content, target, tags, is_public,
		       created_at, updated_at
		FROM templates
		WHERE id = $1 AND is_public = true
	`

	tpl := &Template{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&tpl.ID,
		&tpl.Name,
		&tpl.Description,
		&tpl.Content,
		&tpl.Target,
		&tpl.Tags,
		&tpl.IsPublic,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("模板不存在或未公开: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询公开模板失败: %w", err)
	}

	return tpl, nil
}

// ListPublic 列出所有公开模板（仅返回 id/name/target，不含模板内容）
func (r *TemplateRepo) ListPublic(ctx context.Context) ([]Template, error) {
	query := `
		SELECT id, name, target
		FROM templates
		WHERE is_public = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询公开模板列表失败: %w", err)
	}
	defer rows.Close()

	tpls := []Template{}
	for rows.Next() {
		tpl := Template{}
		if err := rows.Scan(&tpl.ID, &tpl.Name, &tpl.Target); err != nil {
			return nil, fmt.Errorf("扫描公开模板行失败: %w", err)
		}
		tpls = append(tpls, tpl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历公开模板行失败: %w", err)
	}

	return tpls, nil
}

// Update 更新模板，支持修改模板名称，并同步更新策略引用
func (r *TemplateRepo) Update(ctx context.Context, id int64, tpl *Template) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldName string
	if err := tx.QueryRow(ctx, `SELECT name FROM templates WHERE id = $1`, id).Scan(&oldName); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("模板不存在: %d", id)
		}
		return fmt.Errorf("查询模板失败: %w", err)
	}

	query := `
		UPDATE templates
		SET name = $2, description = $3, content = $4, target = $5, tags = $6, is_public = $7
		WHERE id = $1
		RETURNING updated_at
	`

	err = tx.QueryRow(ctx, query,
		id, tpl.Name, tpl.Description, tpl.Content, tpl.Target, tpl.Tags, tpl.IsPublic,
	).Scan(&tpl.UpdatedAt)

	if err == pgx.ErrNoRows {
		return fmt.Errorf("模板不存在: %d", id)
	}
	if err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}

	if oldName != tpl.Name {
		if _, err := tx.Exec(ctx, `
			UPDATE config_policies
			SET template_name = $2
			WHERE template_name = $1
		`, oldName, tpl.Name); err != nil {
			return fmt.Errorf("同步更新策略模板引用失败: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// Delete 删除模板
func (r *TemplateRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM templates WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("模板不存在: %d", id)
	}

	return nil
}

// Exists 检查模板是否存在
func (r *TemplateRepo) Exists(ctx context.Context, name string) (bool, error) {
	query := `SELECT 1 FROM templates WHERE name = $1`

	var exists int
	err := r.db.Pool.QueryRow(ctx, query, name).Scan(&exists)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查模板存在性失败: %w", err)
	}

	return true, nil
}

// GetDB 获取数据库实例
func (r *TemplateRepo) GetDB() *DB {
	return r.db
}
