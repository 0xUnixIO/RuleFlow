-- 1. 将 clash / clash_meta / clash-meta 统一重命名为 clash-mihomo，sing_box 统一为 sing-box
-- 2. 为模板表添加 is_public 字段，控制是否在 /converter 页面对外展示

BEGIN;

-- templates 表：重命名 target 值
ALTER TABLE templates DROP CONSTRAINT templates_target_check;

UPDATE templates SET target = 'clash-mihomo' WHERE target IN ('clash', 'clash_meta');
UPDATE templates SET target = 'sing-box'     WHERE target = 'sing_box';

ALTER TABLE templates
    ALTER COLUMN target SET DEFAULT 'clash-mihomo',
    ADD CONSTRAINT templates_target_check
        CHECK (target IN ('clash-mihomo', 'stash', 'surge', 'sing-box', 'loon', 'shadowrocket'));

-- templates 表：添加 is_public 字段
ALTER TABLE templates ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT false;

-- config_policies 表：重命名 target 值
ALTER TABLE config_policies DROP CONSTRAINT config_policies_target_check;

UPDATE config_policies SET target = 'clash-mihomo' WHERE target IN ('clash', 'clash-meta', 'clash_meta');

ALTER TABLE config_policies
    ALTER COLUMN target SET DEFAULT 'clash-mihomo',
    ADD CONSTRAINT config_policies_target_check
        CHECK (target IN ('clash-mihomo', 'stash', 'surge', 'sing-box'));

COMMIT;
