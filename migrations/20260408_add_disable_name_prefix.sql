-- 为订阅表添加"禁用节点名称前缀"开关
ALTER TABLE subscriptions ADD COLUMN disable_name_prefix BOOLEAN NOT NULL DEFAULT false;
