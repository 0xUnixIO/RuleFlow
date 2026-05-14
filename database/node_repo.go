package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Node 节点模型
type Node struct {
	ID           int64                  `json:"id"`
	Name         string                 `json:"name"`
	Protocol     string                 `json:"protocol"`
	Server       string                 `json:"server"`
	Port         int                    `json:"port"`
	Config       map[string]interface{} `json:"config"`
	SourceID     *int64                 `json:"source_id"`
	SourceName   string                 `json:"source_name"` // 关联查询得到的订阅名称
	Enabled      bool                   `json:"enabled"`
	Tags         []string               `json:"tags"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastSyncedAt *time.Time             `json:"last_synced_at"`
}

// NodeFilter 节点筛选条件
type NodeFilter struct {
	ManualOnly bool     // 仅手动节点
	SourceID   *int64   // 按来源 ID 筛选
	Protocol   string   // 按协议筛选
	Enabled    *bool    // 按启用状态筛选
	Tags       []string // 按标签筛选（OR 关系）
	IDs        []int64  // 按 ID 筛选（精确匹配）
}

// NodeRepo 节点仓储
type NodeRepo struct {
	db *DB
}

// NewNodeRepo 创建节点仓储
func NewNodeRepo(db *DB) *NodeRepo {
	return &NodeRepo{db: db}
}

// Create 创建节点
func (r *NodeRepo) Create(ctx context.Context, node *Node) error {
	node.ID = NextID()
	query := `
		INSERT INTO nodes (id, name, protocol, server, port, config, source_id, enabled, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		node.ID,
		node.Name,
		node.Protocol,
		node.Server,
		node.Port,
		node.Config,
		node.SourceID,
		node.Enabled,
		node.Tags,
	).Scan(&node.CreatedAt, &node.UpdatedAt)

	if err != nil {
		// 检查唯一约束冲突
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return fmt.Errorf("节点已存在: %s", node.Name)
		}
		return fmt.Errorf("创建节点失败: %w", err)
	}

	return nil
}

// UpsertManualImported 在手动导入时按业务唯一键更新已有节点。
// 重复导入时只覆盖协议与配置，保留已有节点的启用状态和标签。
func (r *NodeRepo) UpsertManualImported(ctx context.Context, node *Node) (bool, error) {
	node.ID = NextID()
	query := `
		INSERT INTO nodes (id, name, protocol, server, port, config, source_id, enabled, tags)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8)
		ON CONFLICT (name, server, port) WHERE source_id IS NULL
		DO UPDATE SET
			protocol = EXCLUDED.protocol,
			config = EXCLUDED.config
		RETURNING created_at, updated_at, xmax = 0 AS inserted
	`

	var inserted bool
	err := r.db.Pool.QueryRow(ctx, query,
		node.ID,
		node.Name,
		node.Protocol,
		node.Server,
		node.Port,
		node.Config,
		node.Enabled,
		node.Tags,
	).Scan(&node.CreatedAt, &node.UpdatedAt, &inserted)

	if err != nil {
		return false, fmt.Errorf("导入节点失败: %w", err)
	}

	return inserted, nil
}

// GetByID 根据 ID 获取节点
func (r *NodeRepo) GetByID(ctx context.Context, id int64) (*Node, error) {
	query := `
		SELECT n.id, n.name, n.protocol, n.server, n.port, n.config, n.source_id,
		       COALESCE(s.name, '') AS source_name,
		       n.enabled, n.tags, n.created_at, n.updated_at, n.last_synced_at
		FROM nodes n
		LEFT JOIN subscriptions s ON n.source_id = s.id
		WHERE n.id = $1
	`

	node := &Node{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&node.ID,
		&node.Name,
		&node.Protocol,
		&node.Server,
		&node.Port,
		&node.Config,
		&node.SourceID,
		&node.SourceName,
		&node.Enabled,
		&node.Tags,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.LastSyncedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("节点不存在: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %w", err)
	}

	return node, nil
}

// List 根据筛选条件列出节点
func (r *NodeRepo) List(ctx context.Context, filter NodeFilter) ([]Node, error) {
	query := `
		SELECT n.id, n.name, n.protocol, n.server, n.port, n.config, n.source_id,
		       COALESCE(s.name, '') AS source_name,
		       n.enabled, n.tags, n.created_at, n.updated_at, n.last_synced_at
		FROM nodes n
		LEFT JOIN subscriptions s ON n.source_id = s.id
		WHERE 1=1
	`
	args := []interface{}{}
	argPos := 1

	// 仅手动节点
	if filter.ManualOnly {
		query += " AND n.source_id IS NULL"
	}

	// 按来源 ID 筛选
	if filter.SourceID != nil {
		query += fmt.Sprintf(" AND n.source_id = $%d", argPos)
		args = append(args, *filter.SourceID)
		argPos++
	}

	// 按协议筛选
	if filter.Protocol != "" {
		query += fmt.Sprintf(" AND n.protocol = $%d", argPos)
		args = append(args, filter.Protocol)
		argPos++
	}

	// 按启用状态筛选
	if filter.Enabled != nil {
		query += fmt.Sprintf(" AND n.enabled = $%d", argPos)
		args = append(args, *filter.Enabled)
		argPos++
	}

	// 按 ID 筛选
	if len(filter.IDs) > 0 {
		query += fmt.Sprintf(" AND n.id = ANY($%d)", argPos)
		args = append(args, filter.IDs)
		argPos++
	}

	// 按标签筛选（OR 关系：包含任一标签即可）
	if len(filter.Tags) > 0 {
		query += " AND ("
		for i, tag := range filter.Tags {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf(" $%d = ANY(n.tags)", argPos)
			args = append(args, tag)
			argPos++
		}
		query += ")"
	}

	query += " ORDER BY n.created_at DESC"

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询节点列表失败: %w", err)
	}
	defer rows.Close()

	nodes := []Node{}
	for rows.Next() {
		node := Node{}
		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Protocol,
			&node.Server,
			&node.Port,
			&node.Config,
			&node.SourceID,
			&node.SourceName,
			&node.Enabled,
			&node.Tags,
			&node.CreatedAt,
			&node.UpdatedAt,
			&node.LastSyncedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描节点行失败: %w", err)
		}
		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历节点行失败: %w", err)
	}

	return nodes, nil
}

// Update 更新节点
func (r *NodeRepo) Update(ctx context.Context, node *Node) error {
	query := `
		UPDATE nodes
		SET name = $2, protocol = $3, server = $4, port = $5,
		    config = $6, enabled = $7, tags = $8
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		node.ID,
		node.Name,
		node.Protocol,
		node.Server,
		node.Port,
		node.Config,
		node.Enabled,
		node.Tags,
	).Scan(&node.UpdatedAt)

	if err == pgx.ErrNoRows {
		return fmt.Errorf("节点不存在: %d", node.ID)
	}
	if err != nil {
		return fmt.Errorf("更新节点失败: %w", err)
	}

	return nil
}

// Delete 删除节点
func (r *NodeRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM nodes WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除节点失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("节点不存在: %d", id)
	}

	return nil
}

// DeleteBySourceID 根据来源 ID 删除节点（订阅改名后同步使用）
func (r *NodeRepo) DeleteBySourceID(ctx context.Context, sourceID int64) (int64, error) {
	query := `DELETE FROM nodes WHERE source_id = $1`

	result, err := r.db.Pool.Exec(ctx, query, sourceID)
	if err != nil {
		return 0, fmt.Errorf("按来源 ID 删除节点失败: %w", err)
	}

	return result.RowsAffected(), nil
}

// BatchCreate 批量创建节点
func (r *NodeRepo) BatchCreate(ctx context.Context, nodes []Node) error {
	if len(nodes) == 0 {
		return nil
	}
	return r.batchInsert(ctx, dedupeNodes(nodes))
}

// batchInsert 批量插入（备用方案）
func (r *NodeRepo) batchInsert(ctx context.Context, nodes []Node) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO nodes (id, name, protocol, server, port, config, source_id, enabled, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, node := range nodes {
		node.ID = NextID()
		_, err := tx.Exec(ctx, query,
			node.ID,
			node.Name,
			node.Protocol,
			node.Server,
			node.Port,
			node.Config,
			node.SourceID,
			node.Enabled,
			node.Tags,
		)
		if err != nil {
			return fmt.Errorf("批量插入节点失败: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// dedupeNodes 在批量插入前按业务唯一键去重，避免依赖数据库冲突约束。
func dedupeNodes(nodes []Node) []Node {
	seen := make(map[string]struct{}, len(nodes))
	result := make([]Node, 0, len(nodes))

	for _, node := range nodes {
		sourceKey := "manual"
		if node.SourceID != nil {
			sourceKey = fmt.Sprintf("subscription:%d", *node.SourceID)
		}
		key := strings.Join([]string{
			sourceKey,
			node.Name,
			node.Server,
			fmt.Sprintf("%d", node.Port),
		}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, node)
	}

	return result
}

// BatchUpdateEnabled 批量更新启用状态
func (r *NodeRepo) BatchUpdateEnabled(ctx context.Context, ids []int64, enabled bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	query := `
		UPDATE nodes
		SET enabled = $1
		WHERE id = ANY($2)
	`

	result, err := r.db.Pool.Exec(ctx, query, enabled, ids)
	if err != nil {
		return 0, fmt.Errorf("批量更新节点状态失败: %w", err)
	}

	return result.RowsAffected(), nil
}

// CountBySourceID 统计指定来源 ID 的节点数量
func (r *NodeRepo) CountBySourceID(ctx context.Context, sourceID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM nodes WHERE source_id = $1`

	var count int64
	err := r.db.Pool.QueryRow(ctx, query, sourceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("按来源 ID 统计节点数量失败: %w", err)
	}

	return count, nil
}

// GetDB 获取数据库实例
func (r *NodeRepo) GetDB() *DB {
	return r.db
}
