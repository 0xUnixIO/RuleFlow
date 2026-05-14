-- 数据库备份功能：配置表 + 备份历史记录表

CREATE TABLE IF NOT EXISTS backup_settings (
    id INTEGER PRIMARY KEY DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT false,
    r2_account_id TEXT NOT NULL DEFAULT '',
    r2_access_key_id TEXT NOT NULL DEFAULT '',
    r2_secret_access_key TEXT NOT NULL DEFAULT '',
    r2_bucket_name TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT backup_settings_singleton CHECK (id = 1)
);

INSERT INTO backup_settings (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS backup_records (
    id BIGSERIAL PRIMARY KEY,
    file_key TEXT NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_backup_records_created_at ON backup_records(created_at DESC);
