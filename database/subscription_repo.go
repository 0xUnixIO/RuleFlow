package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SubscriptionFilter 订阅源节点过滤规则
type SubscriptionFilter struct {
	ExcludeKeywords  []string `json:"exclude_keywords,omitempty"`
	ExcludeRegex     string   `json:"exclude_regex,omitempty"`
	IncludeProtocols []string `json:"include_protocols,omitempty"`
}

// UserInfo 订阅流量信息（来自响应头 Subscription-Userinfo）
type UserInfo struct {
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Total    int64  `json:"total"`
	Expire   *int64 `json:"expire,omitempty"`
}

// Subscription 订阅模型
type Subscription struct {
	ID                int64               `json:"id"`
	Name              string              `json:"name"`
	URL               *string             `json:"url"`
	Enabled           bool                `json:"enabled"`
	AutoRefresh       bool                `json:"auto_refresh"`
	RefreshInterval   int                 `json:"refresh_interval"`
	Description       string              `json:"description"`
	Tags              []string            `json:"tags"`
	LastFetchedAt     *time.Time          `json:"last_fetched_at"`
	LastFetchError    *string             `json:"last_fetch_error"`
	NodeCount         int                 `json:"node_count"`
	FilterRules       *SubscriptionFilter `json:"filter_rules,omitempty"`
	DisableNamePrefix bool                `json:"disable_name_prefix"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	UserInfo          *UserInfo           `json:"userinfo,omitempty"`
}

// SubscriptionRepo 订阅仓储
type SubscriptionRepo struct {
	db *DB
}

// NewSubscriptionRepo 创建订阅仓储
func NewSubscriptionRepo(db *DB) *SubscriptionRepo {
	return &SubscriptionRepo{db: db}
}

// scanSubscription 从 scan 函数中读取订阅字段
func scanSubscription(scan func(...any) error) (*Subscription, error) {
	sub := &Subscription{}
	var filterRulesJSON []byte
	var userInfoJSON []byte
	err := scan(
		&sub.ID, &sub.Name, &sub.URL, &sub.Enabled, &sub.AutoRefresh,
		&sub.RefreshInterval, &sub.Description, &sub.Tags,
		&sub.LastFetchedAt, &sub.LastFetchError, &sub.NodeCount,
		&filterRulesJSON, &userInfoJSON, &sub.DisableNamePrefix,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(filterRulesJSON) > 0 {
		sub.FilterRules = &SubscriptionFilter{}
		if err := json.Unmarshal(filterRulesJSON, sub.FilterRules); err != nil {
			return nil, fmt.Errorf("解析过滤规则失败: %w", err)
		}
	}
	if len(userInfoJSON) > 0 {
		sub.UserInfo = &UserInfo{}
		if err := json.Unmarshal(userInfoJSON, sub.UserInfo); err != nil {
			return nil, fmt.Errorf("解析流量信息失败: %w", err)
		}
	}
	return sub, nil
}

const selectSubFields = `
	SELECT id, name, url, enabled, auto_refresh, refresh_interval, description, tags,
	       last_fetched_at, last_fetch_error, node_count, filter_rules, userinfo, disable_name_prefix, created_at, updated_at
	FROM subscriptions
`

// Create 创建订阅
func (r *SubscriptionRepo) Create(ctx context.Context, sub *Subscription) error {
	filterRulesJSON, err := json.Marshal(sub.FilterRules)
	if err != nil {
		return fmt.Errorf("序列化过滤规则失败: %w", err)
	}
	sub.ID = NextID()

	query := `
		INSERT INTO subscriptions (id, name, url, enabled, auto_refresh, refresh_interval, description, tags, filter_rules, disable_name_prefix)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		sub.ID, sub.Name, sub.URL, sub.Enabled, sub.AutoRefresh, sub.RefreshInterval,
		sub.Description, sub.Tags, filterRulesJSON, sub.DisableNamePrefix,
	).Scan(&sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建订阅失败: %w", err)
	}

	return nil
}

// GetByName 根据名称获取订阅
func (r *SubscriptionRepo) GetByName(ctx context.Context, name string) (*Subscription, error) {
	query := selectSubFields + `WHERE name = $1`
	sub, err := scanSubscription(r.db.Pool.QueryRow(ctx, query, name).Scan)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("订阅不存在: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}
	return sub, nil
}

// GetByID 根据 ID 获取订阅
func (r *SubscriptionRepo) GetByID(ctx context.Context, id int64) (*Subscription, error) {
	query := selectSubFields + `WHERE id = $1`
	sub, err := scanSubscription(r.db.Pool.QueryRow(ctx, query, id).Scan)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("订阅不存在: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询订阅失败: %w", err)
	}
	return sub, nil
}

// List 列出所有订阅
func (r *SubscriptionRepo) List(ctx context.Context) ([]Subscription, error) {
	query := selectSubFields + `ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询订阅列表失败: %w", err)
	}
	defer rows.Close()

	subs := []Subscription{}
	for rows.Next() {
		sub, err := scanSubscription(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("扫描订阅行失败: %w", err)
		}
		subs = append(subs, *sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历订阅行失败: %w", err)
	}

	return subs, nil
}

// Update 更新订阅
func (r *SubscriptionRepo) Update(ctx context.Context, sub *Subscription) error {
	filterRulesJSON, err := json.Marshal(sub.FilterRules)
	if err != nil {
		return fmt.Errorf("序列化过滤规则失败: %w", err)
	}

	query := `
		UPDATE subscriptions
		SET name = $2, url = $3, enabled = $4, auto_refresh = $5, refresh_interval = $6,
		    description = $7, tags = $8, filter_rules = $9, disable_name_prefix = $10
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		sub.ID, sub.Name, sub.URL, sub.Enabled, sub.AutoRefresh, sub.RefreshInterval,
		sub.Description, sub.Tags, filterRulesJSON, sub.DisableNamePrefix,
	).Scan(&sub.UpdatedAt)

	if err == pgx.ErrNoRows {
		return fmt.Errorf("订阅不存在: %d", sub.ID)
	}
	if err != nil {
		return fmt.Errorf("更新订阅失败: %w", err)
	}

	return nil
}

// DeleteByID 删除订阅
func (r *SubscriptionRepo) DeleteByID(ctx context.Context, id int64) error {
	query := `DELETE FROM subscriptions WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除订阅失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("订阅不存在: %d", id)
	}

	return nil
}

// UpdateFetchResult 更新订阅获取结果
func (r *SubscriptionRepo) UpdateFetchResult(ctx context.Context, name string, nodeCount int, fetchErr error) error {
	query := `
		UPDATE subscriptions
		SET last_fetched_at = CURRENT_TIMESTAMP,
		    node_count = $2,
		    last_fetch_error = $3
		WHERE name = $1
	`

	var errorMsg *string
	if fetchErr != nil {
		msg := fetchErr.Error()
		errorMsg = &msg
	}

	_, err := r.db.Pool.Exec(ctx, query, name, nodeCount, errorMsg)
	if err != nil {
		return fmt.Errorf("更新获取结果失败: %w", err)
	}

	return nil
}

// UpdateFetchResultByID 更新订阅获取结果
func (r *SubscriptionRepo) UpdateFetchResultByID(ctx context.Context, id int64, nodeCount int, fetchErr error) error {
	query := `
		UPDATE subscriptions
		SET last_fetched_at = CURRENT_TIMESTAMP,
		    node_count = $2,
		    last_fetch_error = $3
		WHERE id = $1
	`

	var errorMsg *string
	if fetchErr != nil {
		msg := fetchErr.Error()
		errorMsg = &msg
	}

	_, err := r.db.Pool.Exec(ctx, query, id, nodeCount, errorMsg)
	if err != nil {
		return fmt.Errorf("更新获取结果失败: %w", err)
	}

	return nil
}

// UpdateUserInfoByID 更新订阅流量信息
func (r *SubscriptionRepo) UpdateUserInfoByID(ctx context.Context, id int64, info *UserInfo) error {
	var userInfoJSON []byte
	var err error
	if info != nil {
		userInfoJSON, err = json.Marshal(info)
		if err != nil {
			return fmt.Errorf("序列化流量信息失败: %w", err)
		}
	}

	query := `UPDATE subscriptions SET userinfo = $2 WHERE id = $1`
	if _, err := r.db.Pool.Exec(ctx, query, id, userInfoJSON); err != nil {
		return fmt.Errorf("更新流量信息失败: %w", err)
	}
	return nil
}

// Exists 检查订阅是否存在
func (r *SubscriptionRepo) Exists(ctx context.Context, name string) (bool, error) {
	query := `SELECT 1 FROM subscriptions WHERE name = $1`

	var exists int
	err := r.db.Pool.QueryRow(ctx, query, name).Scan(&exists)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查订阅存在性失败: %w", err)
	}

	return true, nil
}

// GetDB 获取数据库实例
func (r *SubscriptionRepo) GetDB() *DB {
	return r.db
}
