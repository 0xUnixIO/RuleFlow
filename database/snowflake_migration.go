package database

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
)

const legacyIDMax = int64(math.MaxInt32)

type SnowflakeMigrationReport struct {
	Subscriptions    int  `json:"subscriptions"`
	Templates        int  `json:"templates"`
	RuleSources      int  `json:"rule_sources"`
	ConfigPolicies   int  `json:"config_policies"`
	ConfigAccessLogs int  `json:"config_access_logs"`
	Nodes            int  `json:"nodes"`
	SchemaUpdated    bool `json:"schema_updated"`
	AlreadyMigrated  bool `json:"already_migrated"`
}

func (db *DB) MigrateLegacyIDsToSnowflake(ctx context.Context) (*SnowflakeMigrationReport, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("开始迁移事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := lockMigrationTables(ctx, tx); err != nil {
		return nil, err
	}
	if err := ensureSnowflakeSchema(ctx, tx); err != nil {
		return nil, err
	}

	report := &SnowflakeMigrationReport{SchemaUpdated: true}

	subscriptionMap, err := buildIDMap(ctx, tx, "subscriptions")
	if err != nil {
		return nil, err
	}
	templateMap, err := buildIDMap(ctx, tx, "templates")
	if err != nil {
		return nil, err
	}
	ruleSourceMap, err := buildIDMap(ctx, tx, "rule_sources")
	if err != nil {
		return nil, err
	}
	policyMap, err := buildIDMap(ctx, tx, "config_policies")
	if err != nil {
		return nil, err
	}
	nodeMap, err := buildIDMap(ctx, tx, "nodes")
	if err != nil {
		return nil, err
	}
	accessLogMap, err := buildIDMap(ctx, tx, "config_access_logs")
	if err != nil {
		return nil, err
	}

	report.Subscriptions = len(subscriptionMap)
	report.Templates = len(templateMap)
	report.RuleSources = len(ruleSourceMap)
	report.ConfigPolicies = len(policyMap)
	report.Nodes = len(nodeMap)
	report.ConfigAccessLogs = len(accessLogMap)
	report.AlreadyMigrated = report.Subscriptions == 0 &&
		report.Templates == 0 &&
		report.RuleSources == 0 &&
		report.ConfigPolicies == 0 &&
		report.Nodes == 0 &&
		report.ConfigAccessLogs == 0

	if err := createTempIDMapTable(ctx, tx, "subscription_id_map"); err != nil {
		return nil, err
	}
	if err := createTempIDMapTable(ctx, tx, "template_id_map"); err != nil {
		return nil, err
	}
	if err := createTempIDMapTable(ctx, tx, "rule_source_id_map"); err != nil {
		return nil, err
	}
	if err := createTempIDMapTable(ctx, tx, "config_policy_id_map"); err != nil {
		return nil, err
	}
	if err := createTempIDMapTable(ctx, tx, "node_id_map"); err != nil {
		return nil, err
	}
	if err := createTempIDMapTable(ctx, tx, "config_access_log_id_map"); err != nil {
		return nil, err
	}

	if err := insertIDMap(ctx, tx, "subscription_id_map", subscriptionMap); err != nil {
		return nil, err
	}
	if err := insertIDMap(ctx, tx, "template_id_map", templateMap); err != nil {
		return nil, err
	}
	if err := insertIDMap(ctx, tx, "rule_source_id_map", ruleSourceMap); err != nil {
		return nil, err
	}
	if err := insertIDMap(ctx, tx, "config_policy_id_map", policyMap); err != nil {
		return nil, err
	}
	if err := insertIDMap(ctx, tx, "node_id_map", nodeMap); err != nil {
		return nil, err
	}
	if err := insertIDMap(ctx, tx, "config_access_log_id_map", accessLogMap); err != nil {
		return nil, err
	}

	if err := executeMigrationUpdates(ctx, tx); err != nil {
		return nil, err
	}
	if err := restoreMigrationConstraints(ctx, tx); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("提交迁移事务失败: %w", err)
	}

	return report, nil
}

func lockMigrationTables(ctx context.Context, tx pgx.Tx) error {
	query := `
		LOCK TABLE subscriptions, templates, rule_sources, config_policies, config_access_logs, nodes
		IN ACCESS EXCLUSIVE MODE
	`
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("锁定迁移表失败: %w", err)
	}
	return nil
}

func ensureSnowflakeSchema(ctx context.Context, tx pgx.Tx) error {
	queries := []string{
		`ALTER TABLE config_access_logs DROP CONSTRAINT IF EXISTS config_access_logs_policy_id_fkey`,
		`ALTER TABLE subscriptions ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE templates ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE rule_sources ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE config_policies ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE config_policies ALTER COLUMN subscription_ids TYPE BIGINT[] USING subscription_ids::BIGINT[]`,
		`ALTER TABLE config_policies ALTER COLUMN node_ids TYPE BIGINT[] USING node_ids::BIGINT[]`,
		`ALTER TABLE nodes ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE nodes ALTER COLUMN source_id TYPE BIGINT`,
		`ALTER TABLE config_access_logs ALTER COLUMN id TYPE BIGINT`,
		`ALTER TABLE config_access_logs ALTER COLUMN policy_id TYPE BIGINT`,
	}
	for _, query := range queries {
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("更新雪花 ID 表结构失败: %w", err)
		}
	}
	return nil
}

func buildIDMap(ctx context.Context, tx pgx.Tx, table string) (map[int64]int64, error) {
	query := fmt.Sprintf(`SELECT id FROM %s WHERE id <= $1 ORDER BY id`, table)
	rows, err := tx.Query(ctx, query, legacyIDMax)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 旧 ID 失败: %w", table, err)
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var oldID int64
		if err := rows.Scan(&oldID); err != nil {
			return nil, fmt.Errorf("扫描 %s 旧 ID 失败: %w", table, err)
		}
		result[oldID] = NextID()
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 %s 旧 ID 失败: %w", table, err)
	}

	return result, nil
}

func createTempIDMapTable(ctx context.Context, tx pgx.Tx, table string) error {
	query := fmt.Sprintf(`
		CREATE TEMP TABLE %s (
			old_id BIGINT PRIMARY KEY,
			new_id BIGINT NOT NULL UNIQUE
		) ON COMMIT DROP
	`, table)
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("创建临时映射表 %s 失败: %w", table, err)
	}
	return nil
}

func insertIDMap(ctx context.Context, tx pgx.Tx, table string, idMap map[int64]int64) error {
	if len(idMap) == 0 {
		return nil
	}
	query := fmt.Sprintf(`INSERT INTO %s (old_id, new_id) VALUES ($1, $2)`, table)
	for oldID, newID := range idMap {
		if _, err := tx.Exec(ctx, query, oldID, newID); err != nil {
			return fmt.Errorf("写入临时映射表 %s 失败: %w", table, err)
		}
	}
	return nil
}

func executeMigrationUpdates(ctx context.Context, tx pgx.Tx) error {
	queries := []string{
		`UPDATE subscriptions s SET id = m.new_id FROM subscription_id_map m WHERE s.id = m.old_id`,
		`UPDATE nodes n SET source_id = m.new_id FROM subscription_id_map m WHERE n.source_id = m.old_id`,
		`UPDATE templates t SET id = m.new_id FROM template_id_map m WHERE t.id = m.old_id`,
		`UPDATE rule_sources r SET id = m.new_id FROM rule_source_id_map m WHERE r.id = m.old_id`,
		`UPDATE config_policies p SET id = m.new_id FROM config_policy_id_map m WHERE p.id = m.old_id`,
		`UPDATE config_access_logs l SET policy_id = m.new_id FROM config_policy_id_map m WHERE l.policy_id = m.old_id`,
		`UPDATE nodes n SET id = m.new_id FROM node_id_map m WHERE n.id = m.old_id`,
		`
		UPDATE config_policies p
		SET subscription_ids = COALESCE((
			SELECT array_agg(COALESCE(m.new_id, u.id) ORDER BY u.ord)
			FROM unnest(p.subscription_ids) WITH ORDINALITY AS u(id, ord)
			LEFT JOIN subscription_id_map m ON m.old_id = u.id
		), '{}'::BIGINT[])
		WHERE EXISTS (
			SELECT 1
			FROM unnest(p.subscription_ids) AS u(id)
			JOIN subscription_id_map m ON m.old_id = u.id
		)
		`,
		`
		UPDATE config_policies p
		SET node_ids = COALESCE((
			SELECT array_agg(COALESCE(m.new_id, u.id) ORDER BY u.ord)
			FROM unnest(p.node_ids) WITH ORDINALITY AS u(id, ord)
			LEFT JOIN node_id_map m ON m.old_id = u.id
		), '{}'::BIGINT[])
		WHERE EXISTS (
			SELECT 1
			FROM unnest(p.node_ids) AS u(id)
			JOIN node_id_map m ON m.old_id = u.id
		)
		`,
		`UPDATE config_access_logs l SET id = m.new_id FROM config_access_log_id_map m WHERE l.id = m.old_id`,
	}

	for _, query := range queries {
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("刷新雪花 ID 数据失败: %w", err)
		}
	}
	return nil
}

func restoreMigrationConstraints(ctx context.Context, tx pgx.Tx) error {
	query := `
		ALTER TABLE config_access_logs
		ADD CONSTRAINT config_access_logs_policy_id_fkey
		FOREIGN KEY (policy_id) REFERENCES config_policies(id) ON DELETE CASCADE
	`
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("恢复访问日志外键失败: %w", err)
	}
	return nil
}
