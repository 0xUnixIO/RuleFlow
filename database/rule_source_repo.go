package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ablate-ai/RuleFlow/internal/app"
	"github.com/jackc/pgx/v5"
)

type RuleSource struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	URL             string          `json:"url"`
	SourceFormat    string          `json:"source_format"`
	Enabled         bool            `json:"enabled"`
	AutoRefresh     bool            `json:"auto_refresh"`
	RefreshInterval int             `json:"refresh_interval"`
	Tags            []string        `json:"tags"`
	RawContent      string          `json:"raw_content,omitempty"`
	ParsedRules     json.RawMessage `json:"parsed_rules,omitempty"`
	RuleCount       int             `json:"rule_count"`
	LastSyncedAt    *time.Time      `json:"last_synced_at"`
	LastSyncError   *string         `json:"last_sync_error"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type RuleSourceRepo struct {
	db *DB
}

func NewRuleSourceRepo(db *DB) *RuleSourceRepo {
	return &RuleSourceRepo{db: db}
}

const selectRuleSourceFields = `
	SELECT id, name, description, url, source_format, enabled, auto_refresh, refresh_interval,
	       tags, raw_content, parsed_rules, rule_count, last_synced_at, last_sync_error, created_at, updated_at
	FROM rule_sources
`

func scanRuleSource(scan func(...any) error) (*RuleSource, error) {
	source := &RuleSource{}
	var rawContent *string
	err := scan(
		&source.ID,
		&source.Name,
		&source.Description,
		&source.URL,
		&source.SourceFormat,
		&source.Enabled,
		&source.AutoRefresh,
		&source.RefreshInterval,
		&source.Tags,
		&rawContent,
		&source.ParsedRules,
		&source.RuleCount,
		&source.LastSyncedAt,
		&source.LastSyncError,
		&source.CreatedAt,
		&source.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if rawContent != nil {
		source.RawContent = *rawContent
	} else {
		source.RawContent = ""
	}
	if len(source.ParsedRules) == 0 {
		source.ParsedRules = json.RawMessage("[]")
	}
	source.Name = strings.TrimSpace(source.Name)
	return source, nil
}

func (r *RuleSourceRepo) Create(ctx context.Context, source *RuleSource) error {
	source.ID = NextID()
	query := `
		INSERT INTO rule_sources (id, name, description, url, source_format, enabled, auto_refresh, refresh_interval, tags, raw_content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`
	err := r.db.Pool.QueryRow(ctx, query,
		source.ID,
		source.Name,
		source.Description,
		source.URL,
		source.SourceFormat,
		source.Enabled,
		source.AutoRefresh,
		source.RefreshInterval,
		source.Tags,
		source.RawContent,
	).Scan(&source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return fmt.Errorf("创建规则源失败: %w", err)
	}

	r.syncManualRuleSource(ctx, source)

	return nil
}

func (r *RuleSourceRepo) GetByID(ctx context.Context, id int64) (*RuleSource, error) {
	query := selectRuleSourceFields + `WHERE id = $1`
	source, err := scanRuleSource(r.db.Pool.QueryRow(ctx, query, id).Scan)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("规则源不存在: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询规则源失败: %w", err)
	}
	return source, nil
}

func (r *RuleSourceRepo) GetByName(ctx context.Context, name string) (*RuleSource, error) {
	query := selectRuleSourceFields + `WHERE name = $1`
	source, err := scanRuleSource(r.db.Pool.QueryRow(ctx, query, name).Scan)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("规则源不存在: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("查询规则源失败: %w", err)
	}
	return source, nil
}

func (r *RuleSourceRepo) List(ctx context.Context) ([]RuleSource, error) {
	query := selectRuleSourceFields + `ORDER BY created_at DESC`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询规则源列表失败: %w", err)
	}
	defer rows.Close()

	sources := make([]RuleSource, 0)
	for rows.Next() {
		source, err := scanRuleSource(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("扫描规则源失败: %w", err)
		}
		sources = append(sources, *source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历规则源失败: %w", err)
	}
	return sources, nil
}

func (r *RuleSourceRepo) Update(ctx context.Context, source *RuleSource) error {
	query := `
		UPDATE rule_sources
		SET name = $2, description = $3, url = $4, source_format = $5, enabled = $6,
		    auto_refresh = $7, refresh_interval = $8, tags = $9, raw_content = $10
		WHERE id = $1
		RETURNING updated_at
	`
	err := r.db.Pool.QueryRow(ctx, query,
		source.ID,
		source.Name,
		source.Description,
		source.URL,
		source.SourceFormat,
		source.Enabled,
		source.AutoRefresh,
		source.RefreshInterval,
		source.Tags,
		source.RawContent,
	).Scan(&source.UpdatedAt)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("规则源不存在: %d", source.ID)
	}
	if err != nil {
		return fmt.Errorf("更新规则源失败: %w", err)
	}

	r.syncManualRuleSource(ctx, source)

	return nil
}

func (r *RuleSourceRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM rule_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除规则源失败: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("规则源不存在: %d", id)
	}
	return nil
}

func (r *RuleSourceRepo) Exists(ctx context.Context, name string) (bool, error) {
	var exists int
	err := r.db.Pool.QueryRow(ctx, `SELECT 1 FROM rule_sources WHERE name = $1`, name).Scan(&exists)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查规则源失败: %w", err)
	}
	return true, nil
}

func (r *RuleSourceRepo) UpdateSyncResult(ctx context.Context, id int64, rawContent string, parsedRules json.RawMessage, ruleCount int, syncErr error) error {
	var errorMsg *string
	if syncErr != nil {
		msg := syncErr.Error()
		errorMsg = &msg
	}
	query := `
		UPDATE rule_sources
		SET raw_content = $2, parsed_rules = $3, rule_count = $4, last_synced_at = CURRENT_TIMESTAMP, last_sync_error = $5
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, id, rawContent, parsedRules, ruleCount, errorMsg)
	if err != nil {
		return fmt.Errorf("更新规则源同步结果失败: %w", err)
	}
	return nil
}

func ParseRuleSourceContent(rawContent string, sourceFormat string) (json.RawMessage, int, error) {
	rules, err := app.ParseRuleSet(rawContent, sourceFormat)
	if err != nil {
		return json.RawMessage("[]"), 0, err
	}

	parsed, err := json.Marshal(rules)
	if err != nil {
		return nil, 0, err
	}

	return parsed, len(rules), nil
}

func (r *RuleSourceRepo) syncManualRuleSource(ctx context.Context, source *RuleSource) {
	if source.URL != "" || source.RawContent == "" {
		return
	}

	parsed, ruleCount, err := ParseRuleSourceContent(source.RawContent, source.SourceFormat)
	if err != nil {
		return
	}

	_ = r.UpdateSyncResult(ctx, source.ID, source.RawContent, parsed, ruleCount, nil)
}
