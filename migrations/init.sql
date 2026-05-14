-- RuleFlow 完整初始化脚本
-- 适用于每次重新建库后一次性执行

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE subscriptions (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    auto_refresh BOOLEAN NOT NULL DEFAULT false,
    refresh_interval INTEGER NOT NULL DEFAULT 3600,
    description TEXT,
    tags TEXT[] DEFAULT '{}',
    last_fetched_at TIMESTAMP,
    last_fetch_error TEXT,
    node_count INTEGER DEFAULT 0,
    filter_rules JSONB DEFAULT '{}',
    userinfo JSONB,
    disable_name_prefix BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subscriptions_name ON subscriptions(name);
CREATE INDEX idx_subscriptions_enabled ON subscriptions(enabled) WHERE enabled = true;

CREATE TRIGGER update_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON COLUMN subscriptions.auto_refresh IS '是否自动刷新订阅';
COMMENT ON COLUMN subscriptions.refresh_interval IS '自动刷新间隔（秒）';
COMMENT ON COLUMN subscriptions.filter_rules IS '节点过滤规则：exclude_keywords（排除关键词列表）、exclude_regex（排除正则）、include_protocols（协议白名单）';
COMMENT ON COLUMN subscriptions.userinfo IS '订阅流量信息，来自响应头 Subscription-Userinfo';

CREATE TABLE templates (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    content TEXT NOT NULL,
    target VARCHAR(20) NOT NULL DEFAULT 'clash',
    tags TEXT[] DEFAULT '{}',
    is_public BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT templates_target_check
        CHECK (target IN ('clash', 'clash_meta', 'stash', 'surge', 'sing_box', 'loon', 'shadowrocket'))
);

CREATE INDEX idx_templates_name ON templates(name);
CREATE INDEX idx_templates_target ON templates(target);

CREATE TRIGGER update_templates_updated_at
    BEFORE UPDATE ON templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE rule_sources (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    url TEXT,
    source_format VARCHAR(32) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    auto_refresh BOOLEAN NOT NULL DEFAULT false,
    refresh_interval INTEGER NOT NULL DEFAULT 43200,
    tags TEXT[] DEFAULT '{}',
    raw_content TEXT,
    parsed_rules JSONB NOT NULL DEFAULT '[]',
    rule_count INTEGER NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMP,
    last_sync_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT rule_sources_source_format_check
        CHECK (source_format IN ('surge', 'clash-classical', 'clash-domain', 'clash-ipcidr', 'domain-list', 'ip-list'))
);

CREATE INDEX idx_rule_sources_name ON rule_sources(name);
CREATE INDEX idx_rule_sources_enabled ON rule_sources(enabled) WHERE enabled = true;
CREATE INDEX idx_rule_sources_auto_refresh ON rule_sources(auto_refresh) WHERE auto_refresh = true;

CREATE TRIGGER update_rule_sources_updated_at
    BEFORE UPDATE ON rule_sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE config_policies (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    token VARCHAR(64) NOT NULL UNIQUE,
    description TEXT,
    subscription_ids BIGINT[] NOT NULL DEFAULT '{}',
    node_ids BIGINT[] NOT NULL DEFAULT '{}',
    template_name VARCHAR(255),
    target VARCHAR(20) NOT NULL DEFAULT 'clash',
    node_filters JSONB DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    tags TEXT[] DEFAULT '{}',
    last_accessed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT config_policies_target_check
        CHECK (target IN ('clash-meta', 'stash', 'surge', 'sing-box'))
);

CREATE INDEX idx_config_policies_name ON config_policies(name);
CREATE INDEX idx_config_policies_enabled ON config_policies(enabled) WHERE enabled = true;
CREATE INDEX idx_config_policies_tags ON config_policies USING GIN(tags);

CREATE TRIGGER update_config_policies_updated_at
    BEFORE UPDATE ON config_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE config_policies IS '配置生成策略表';
COMMENT ON COLUMN config_policies.subscription_ids IS '关联的订阅源 ID，可以从多个订阅源合并节点';
COMMENT ON COLUMN config_policies.node_ids IS '直接指定的手动节点 ID 列表';
COMMENT ON COLUMN config_policies.template_name IS '使用的规则模板，为空则使用内置模板';
COMMENT ON COLUMN config_policies.node_filters IS '节点过滤条件，JSON 格式存储复杂的过滤逻辑';
COMMENT ON COLUMN config_policies.last_accessed_at IS '用户最近一次成功请求订阅配置的时间';

CREATE TABLE config_access_logs (
    id BIGINT PRIMARY KEY,
    policy_id BIGINT REFERENCES config_policies(id) ON DELETE CASCADE,
    token VARCHAR(64),
    client_ip INET,
    user_agent TEXT,
    status_code INTEGER NOT NULL,
    success BOOLEAN NOT NULL DEFAULT false,
    cache_hit BOOLEAN NOT NULL DEFAULT false,
    node_count INTEGER,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_config_access_logs_policy_id_created_at ON config_access_logs(policy_id, created_at DESC);
CREATE INDEX idx_config_access_logs_created_at ON config_access_logs(created_at DESC);

COMMENT ON TABLE config_access_logs IS '订阅配置访问日志';
COMMENT ON COLUMN config_access_logs.policy_id IS '被访问的配置策略 ID，token 无效时可为空';
COMMENT ON COLUMN config_access_logs.token IS '请求中携带的 token，便于排查无效访问';
COMMENT ON COLUMN config_access_logs.client_ip IS '客户端 IP，优先取反向代理透传头';
COMMENT ON COLUMN config_access_logs.user_agent IS '客户端 User-Agent';
COMMENT ON COLUMN config_access_logs.status_code IS 'HTTP 返回状态码';
COMMENT ON COLUMN config_access_logs.success IS '是否成功生成或返回配置';
COMMENT ON COLUMN config_access_logs.cache_hit IS '是否命中配置缓存';
COMMENT ON COLUMN config_access_logs.node_count IS '本次返回的节点数，失败时可为空';
COMMENT ON COLUMN config_access_logs.error_message IS '失败原因';

CREATE TABLE nodes (
    id BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    protocol VARCHAR(20) NOT NULL,
    server VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    config JSONB NOT NULL,
    source_id BIGINT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_synced_at TIMESTAMP,
    CONSTRAINT nodes_protocol_check
        CHECK (protocol IN ('trojan', 'vmess', 'vless', 'ss', 'wireguard', 'anytls', 'hysteria2', 'tuic'))
);

CREATE INDEX idx_nodes_source_id ON nodes(source_id) WHERE source_id IS NOT NULL;
CREATE INDEX idx_nodes_protocol ON nodes(protocol);
CREATE INDEX idx_nodes_enabled ON nodes(enabled) WHERE enabled = true;
CREATE INDEX idx_nodes_tags ON nodes USING GIN(tags);
CREATE UNIQUE INDEX uq_nodes_manual_identity ON nodes(name, server, port) WHERE source_id IS NULL;
CREATE UNIQUE INDEX uq_nodes_subscription_identity ON nodes(source_id, name, server, port) WHERE source_id IS NOT NULL;

CREATE TRIGGER update_nodes_updated_at
    BEFORE UPDATE ON nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE nodes IS '节点表，存储订阅和手动添加的代理节点';
COMMENT ON COLUMN nodes.protocol IS '协议类型：trojan, vmess, vless, ss, wireguard, anytls, hysteria2, tuic';
COMMENT ON COLUMN nodes.config IS '协议特定配置（密码、UUID 等），JSON 格式';
COMMENT ON COLUMN nodes.source_id IS '订阅 ID；为空表示手动添加节点';
COMMENT ON COLUMN nodes.last_synced_at IS '最后同步时间（订阅节点）';
