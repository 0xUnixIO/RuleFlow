package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ConfigPolicy 配置策略
type ConfigPolicy struct {
	ID              int64                  `json:"id"`
	Name            string                 `json:"name"`
	Token           string                 `json:"token"`
	Description     string                 `json:"description"`
	SubscriptionIDs []int64                `json:"subscription_ids"`
	NodeIDs         []int64                `json:"node_ids"`
	TemplateName    string                 `json:"template_name"`
	Target          string                 `json:"target"`
	NodeFilters     map[string]interface{} `json:"node_filters"`
	Enabled         bool                   `json:"enabled"`
	Tags            []string               `json:"tags"`
	LastAccessedAt  *time.Time             `json:"last_accessed_at"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ConfigPolicyRepo 配置策略仓储
type ConfigPolicyRepo struct {
	db *DB
}

// NewConfigPolicyRepo 创建配置策略仓储
func NewConfigPolicyRepo(db *DB) *ConfigPolicyRepo {
	return &ConfigPolicyRepo{db: db}
}

// generateToken 生成随机访问 token
func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ensureInt64Slice(values []int64) []int64 {
	if values == nil {
		return []int64{}
	}
	return values
}

func ensureStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// Create 创建配置策略，自动生成访问 token
func (r *ConfigPolicyRepo) Create(ctx context.Context, policy *ConfigPolicy) error {
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("生成 token 失败: %w", err)
	}
	policy.ID = NextID()
	policy.Token = token

	nodeFiltersJSON, err := json.Marshal(policy.NodeFilters)
	if err != nil {
		return fmt.Errorf("序列化节点过滤条件失败: %w", err)
	}

	query := `
		INSERT INTO config_policies (id, name, token, description, subscription_ids, node_ids, template_name, target, node_filters, enabled, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		policy.ID,
		policy.Name,
		policy.Token,
		policy.Description,
		ensureInt64Slice(policy.SubscriptionIDs),
		ensureInt64Slice(policy.NodeIDs),
		policy.TemplateName,
		policy.Target,
		nodeFiltersJSON,
		policy.Enabled,
		ensureStringSlice(policy.Tags),
	).Scan(&policy.CreatedAt, &policy.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建配置策略失败: %w", err)
	}

	return nil
}

// scanPolicy 扫描一行策略数据
func scanPolicy(scan func(...any) error) (*ConfigPolicy, error) {
	policy := &ConfigPolicy{}
	var nodeFiltersJSON []byte

	err := scan(
		&policy.ID,
		&policy.Name,
		&policy.Token,
		&policy.Description,
		&policy.SubscriptionIDs,
		&policy.NodeIDs,
		&policy.TemplateName,
		&policy.Target,
		&nodeFiltersJSON,
		&policy.Enabled,
		&policy.Tags,
		&policy.LastAccessedAt,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(nodeFiltersJSON) > 0 {
		if err := json.Unmarshal(nodeFiltersJSON, &policy.NodeFilters); err != nil {
			return nil, fmt.Errorf("解析节点过滤条件失败: %w", err)
		}
	}

	return policy, nil
}

const selectPolicyFields = `
	SELECT id, name, token, description, subscription_ids, node_ids, template_name, target, node_filters, enabled, tags, last_accessed_at, created_at, updated_at
	FROM config_policies
`

// GetByName 根据名称获取配置策略
func (r *ConfigPolicyRepo) GetByName(ctx context.Context, name string) (*ConfigPolicy, error) {
	query := selectPolicyFields + `WHERE name = $1`
	policy, err := scanPolicy(r.db.Pool.QueryRow(ctx, query, name).Scan)
	if err != nil {
		return nil, fmt.Errorf("获取配置策略失败: %w", err)
	}
	return policy, nil
}

// GetByID 根据 ID 获取配置策略
func (r *ConfigPolicyRepo) GetByID(ctx context.Context, id int64) (*ConfigPolicy, error) {
	query := selectPolicyFields + `WHERE id = $1`
	policy, err := scanPolicy(r.db.Pool.QueryRow(ctx, query, id).Scan)
	if err != nil {
		return nil, fmt.Errorf("配置策略不存在: %d", id)
	}
	return policy, nil
}

// GetByToken 根据 token 获取配置策略
func (r *ConfigPolicyRepo) GetByToken(ctx context.Context, token string) (*ConfigPolicy, error) {
	query := selectPolicyFields + `WHERE token = $1`
	policy, err := scanPolicy(r.db.Pool.QueryRow(ctx, query, token).Scan)
	if err != nil {
		return nil, fmt.Errorf("配置策略不存在或 token 无效: %w", err)
	}
	return policy, nil
}

// List 获取所有配置策略
func (r *ConfigPolicyRepo) List(ctx context.Context) ([]*ConfigPolicy, error) {
	query := selectPolicyFields + `ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询配置策略列表失败: %w", err)
	}
	defer rows.Close()

	policies := make([]*ConfigPolicy, 0)
	for rows.Next() {
		policy, err := scanPolicy(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("扫描配置策略行失败: %w", err)
		}
		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历配置策略结果失败: %w", err)
	}

	return policies, nil
}

// Update 更新配置策略
func (r *ConfigPolicyRepo) Update(ctx context.Context, policy *ConfigPolicy) error {
	nodeFiltersJSON, err := json.Marshal(policy.NodeFilters)
	if err != nil {
		return fmt.Errorf("序列化节点过滤条件失败: %w", err)
	}

	query := `
		UPDATE config_policies
		SET name = $2, description = $3, subscription_ids = $4, node_ids = $5, template_name = $6, target = $7, node_filters = $8, enabled = $9, tags = $10
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		policy.ID,
		policy.Name,
		policy.Description,
		ensureInt64Slice(policy.SubscriptionIDs),
		ensureInt64Slice(policy.NodeIDs),
		policy.TemplateName,
		policy.Target,
		nodeFiltersJSON,
		policy.Enabled,
		ensureStringSlice(policy.Tags),
	).Scan(&policy.UpdatedAt)

	if err != nil {
		return fmt.Errorf("更新配置策略失败: %w", err)
	}

	return nil
}

// Delete 删除配置策略
func (r *ConfigPolicyRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM config_policies WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除配置策略失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("配置策略不存在: %d", id)
	}

	return nil
}

// GetEnabled 获取所有启用的配置策略
func (r *ConfigPolicyRepo) GetEnabled(ctx context.Context) ([]*ConfigPolicy, error) {
	query := selectPolicyFields + `WHERE enabled = true ORDER BY created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询启用的配置策略列表失败: %w", err)
	}
	defer rows.Close()

	policies := make([]*ConfigPolicy, 0)
	for rows.Next() {
		policy, err := scanPolicy(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("扫描配置策略行失败: %w", err)
		}
		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历配置策略结果失败: %w", err)
	}

	return policies, nil
}

// TouchAccess 更新策略最近访问时间
func (r *ConfigPolicyRepo) TouchAccess(ctx context.Context, id int64) error {
	query := `
		UPDATE config_policies
		SET last_accessed_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("更新策略最近访问时间失败: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("配置策略不存在: %d", id)
	}

	return nil
}
