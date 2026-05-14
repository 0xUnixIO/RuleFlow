package database

import (
	"context"
	"fmt"
	"net/netip"
	"time"
)

// ConfigAccessLog 订阅配置访问日志
type ConfigAccessLog struct {
	ID           int64     `json:"id"`
	PolicyID     *int64    `json:"policy_id,omitempty"`
	PolicyName   string    `json:"policy_name,omitempty"`
	Token        string    `json:"token,omitempty"`
	ClientIP     *string   `json:"client_ip,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	StatusCode   int       `json:"status_code"`
	Success      bool      `json:"success"`
	CacheHit     bool      `json:"cache_hit"`
	NodeCount    *int      `json:"node_count,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ConfigAccessLogRepo 订阅配置访问日志仓储
type ConfigAccessLogRepo struct {
	db *DB
}

// ConfigAccessLogFilter 访问日志筛选条件
type ConfigAccessLogFilter struct {
	PolicyID *int64
	Success  *bool
	CacheHit *bool
}

// NewConfigAccessLogRepo 创建访问日志仓储
func NewConfigAccessLogRepo(db *DB) *ConfigAccessLogRepo {
	return &ConfigAccessLogRepo{db: db}
}

// Create 写入访问日志
func (r *ConfigAccessLogRepo) Create(ctx context.Context, log *ConfigAccessLog) error {
	log.ID = NextID()
	query := `
		INSERT INTO config_access_logs (id, policy_id, token, client_ip, user_agent, status_code, success, cache_hit, node_count, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''))
		RETURNING created_at
	`

	var clientIP any
	if log.ClientIP != nil && *log.ClientIP != "" {
		if parsed, err := netip.ParseAddr(*log.ClientIP); err == nil {
			clientIP = parsed.String()
		}
	}

	err := r.db.Pool.QueryRow(ctx, query,
		log.ID,
		log.PolicyID,
		log.Token,
		clientIP,
		log.UserAgent,
		log.StatusCode,
		log.Success,
		log.CacheHit,
		log.NodeCount,
		log.ErrorMessage,
	).Scan(&log.CreatedAt)
	if err != nil {
		return fmt.Errorf("写入访问日志失败: %w", err)
	}

	return nil
}

// ListByPolicy 按策略查询最近访问日志
func (r *ConfigAccessLogRepo) ListByPolicy(ctx context.Context, policyID int64) ([]*ConfigAccessLog, error) {
	return r.List(ctx, ConfigAccessLogFilter{
		PolicyID: &policyID,
	})
}

// List 查询访问日志
func (r *ConfigAccessLogRepo) List(ctx context.Context, filter ConfigAccessLogFilter) ([]*ConfigAccessLog, error) {
	query := `
		SELECT l.id, l.policy_id, COALESCE(p.name, ''), l.token, host(l.client_ip), l.user_agent, l.status_code, l.success, l.cache_hit, l.node_count, COALESCE(l.error_message, ''), l.created_at
		FROM config_access_logs l
		LEFT JOIN config_policies p ON p.id = l.policy_id
		WHERE ($1::BIGINT IS NULL OR l.policy_id = $1)
		  AND ($2::BOOLEAN IS NULL OR l.success = $2)
		  AND ($3::BOOLEAN IS NULL OR l.cache_hit = $3)
		ORDER BY l.created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, filter.PolicyID, filter.Success, filter.CacheHit)
	if err != nil {
		return nil, fmt.Errorf("查询访问日志失败: %w", err)
	}
	defer rows.Close()

	logs := make([]*ConfigAccessLog, 0)
	for rows.Next() {
		var log ConfigAccessLog
		var policyIDValue *int64
		var clientIP *string
		err := rows.Scan(
			&log.ID,
			&policyIDValue,
			&log.PolicyName,
			&log.Token,
			&clientIP,
			&log.UserAgent,
			&log.StatusCode,
			&log.Success,
			&log.CacheHit,
			&log.NodeCount,
			&log.ErrorMessage,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描访问日志失败: %w", err)
		}
		log.PolicyID = policyIDValue
		log.ClientIP = clientIP
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历访问日志失败: %w", err)
	}

	return logs, nil
}

// CleanupOldLogs 清理旧日志（保留指定天数或最大数量）
func (r *ConfigAccessLogRepo) CleanupOldLogs(ctx context.Context, keepDays int, maxRecords int) (int64, error) {
	var deletedCount int64 = 0

	// 1. 清理超过指定天数的日志
	if keepDays > 0 {
		cutoffTime := time.Now().AddDate(0, 0, -keepDays)
		query := `DELETE FROM config_access_logs WHERE created_at < $1`
		result, err := r.db.Pool.Exec(ctx, query, cutoffTime)
		if err != nil {
			return 0, fmt.Errorf("清理旧日志失败: %w", err)
		}
		count := result.RowsAffected()
		deletedCount += count
	}

	// 2. 限制总记录数（如果记录数超过 maxRecords）
	if maxRecords > 0 {
		// 先查询当前总数
		var totalCount int
		queryCount := `SELECT COUNT(*) FROM config_access_logs`
		err := r.db.Pool.QueryRow(ctx, queryCount).Scan(&totalCount)
		if err != nil {
			return deletedCount, fmt.Errorf("查询日志总数失败: %w", err)
		}

		if totalCount > maxRecords {
			// 计算需要删除的数量
			deleteCount := totalCount - maxRecords
			// 找到需要保留的最新记录的ID
			var minKeepID int64
			queryGetMinID := `SELECT id FROM config_access_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`
			err = r.db.Pool.QueryRow(ctx, queryGetMinID, 1, deleteCount).Scan(&minKeepID)
			if err != nil {
				return deletedCount, fmt.Errorf("查询保留记录ID失败: %w", err)
			}

			// 删除比 minKeepID 小的记录
			queryDelete := `DELETE FROM config_access_logs WHERE id < $1`
			result, err := r.db.Pool.Exec(ctx, queryDelete, minKeepID)
			if err != nil {
				return deletedCount, fmt.Errorf("删除多余记录失败: %w", err)
			}
			count := result.RowsAffected()
			deletedCount += count
		}
	}

	return deletedCount, nil
}
