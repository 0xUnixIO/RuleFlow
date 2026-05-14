package database

import (
	"context"
	"time"
)

// BackupSettings R2 备份配置（单行 singleton 表）
type BackupSettings struct {
	Enabled           bool      `json:"enabled"`
	R2AccountID       string    `json:"r2_account_id"`
	R2AccessKeyID     string    `json:"r2_access_key_id"`
	R2SecretAccessKey string    `json:"r2_secret_access_key"`
	R2BucketName      string    `json:"r2_bucket_name"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// BackupRecord 单次备份记录
type BackupRecord struct {
	ID           int64     `json:"id"`
	FileKey      string    `json:"file_key"`
	FileSize     int64     `json:"file_size"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// BackupRepo 备份数据访问
type BackupRepo struct {
	db *DB
}

func NewBackupRepo(db *DB) *BackupRepo {
	return &BackupRepo{db: db}
}

func (r *BackupRepo) GetSettings(ctx context.Context) (*BackupSettings, error) {
	var s BackupSettings
	err := r.db.Pool.QueryRow(ctx, `
		SELECT enabled, r2_account_id, r2_access_key_id, r2_secret_access_key,
		       r2_bucket_name, updated_at
		FROM backup_settings WHERE id = 1
	`).Scan(&s.Enabled, &s.R2AccountID, &s.R2AccessKeyID, &s.R2SecretAccessKey,
		&s.R2BucketName, &s.UpdatedAt)
	return &s, err
}

func (r *BackupRepo) SaveSettings(ctx context.Context, s *BackupSettings) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO backup_settings (id, enabled, r2_account_id, r2_access_key_id,
		    r2_secret_access_key, r2_bucket_name, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
		    enabled = $1, r2_account_id = $2, r2_access_key_id = $3,
		    r2_secret_access_key = $4, r2_bucket_name = $5, updated_at = NOW()
	`, s.Enabled, s.R2AccountID, s.R2AccessKeyID, s.R2SecretAccessKey, s.R2BucketName)
	return err
}

func (r *BackupRepo) CreateRecord(ctx context.Context, rec *BackupRecord) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO backup_records (file_key, file_size, status, error_message)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, created_at
	`, rec.FileKey, rec.FileSize, rec.Status, rec.ErrorMessage).Scan(&rec.ID, &rec.CreatedAt)
}


// PruneOldRecords 删除超出 keep 数量的旧记录，返回被删除的记录（用于同步删除 R2 文件）
func (r *BackupRepo) PruneOldRecords(ctx context.Context, keep int) ([]*BackupRecord, error) {
	rows, err := r.db.Pool.Query(ctx, `
		DELETE FROM backup_records
		WHERE id NOT IN (
		    SELECT id FROM backup_records ORDER BY created_at DESC LIMIT $1
		)
		RETURNING id, file_key, file_size, status, COALESCE(error_message, ''), created_at
	`, keep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deleted []*BackupRecord
	for rows.Next() {
		var rec BackupRecord
		if err := rows.Scan(&rec.ID, &rec.FileKey, &rec.FileSize, &rec.Status,
			&rec.ErrorMessage, &rec.CreatedAt); err != nil {
			return nil, err
		}
		deleted = append(deleted, &rec)
	}
	return deleted, nil
}
