export interface Subscription {
  id: number;
  name: string;
  url: string;
  enabled: boolean;
  auto_refresh: boolean;
  refresh_interval: number;
  description: string;
  tags: string[];
  last_fetched_at: string | null;
  last_fetch_error: string | null;
  node_count: number;
  filter_rules: {
    exclude_keywords: string[];
    exclude_regex: string;
    include_protocols: string[];
  };
  disable_name_prefix: boolean;
  userinfo: {
    upload: number;
    download: number;
    total: number;
    expire: number | null;
  } | null;
  created_at: string;
  updated_at: string;
}

export interface Template {
  id: number;
  name: string;
  description: string;
  content: string;
  target: string;
  tags: string[];
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export interface Node {
  id: number;
  name: string;
  protocol: string;
  server: string;
  port: number;
  config: Record<string, unknown>;
  source_id: number | null;
  source_name: string;
  enabled: boolean;
  tags: string[];
  created_at: string;
  updated_at: string;
  last_synced_at: string | null;
}

export interface NodeStats {
  total: number;
  enabled: number;
  disabled: number;
  by_protocol: Record<string, number>;
  by_source: Record<string, number>;
}

export interface ConfigPolicy {
  id: number;
  name: string;
  token: string;
  description: string;
  subscription_ids: number[];
  node_ids: number[];
  template_name: string;
  target: string;
  node_filters: Record<string, unknown>;
  enabled: boolean;
  tags: string[];
  last_accessed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface RuleSource {
  id: number;
  name: string;
  description: string;
  url: string;
  source_format: string;
  enabled: boolean;
  auto_refresh: boolean;
  refresh_interval: number;
  tags: string[];
  raw_content: string | null;
  parsed_rules: unknown;
  rule_count: number;
  last_synced_at: string | null;
  last_sync_error: string | null;
  created_at: string;
  updated_at: string;
}

export interface ConfigAccessLog {
  id: number;
  policy_id: number | null;
  token: string;
  client_ip: string | null;
  user_agent: string;
  status_code: number;
  success: boolean;
  cache_hit: boolean;
  node_count: number | null;
  error_message: string;
  created_at: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T;
  error?: string;
}

export interface BackupSettings {
  enabled: boolean;
  r2_account_id: string;
  r2_access_key_id: string;
  r2_secret_access_key: string;
  r2_bucket_name: string;
  updated_at: string;
}

export interface BackupRecord {
  id: number;
  file_key: string;
  file_size: number;
  status: "success" | "failed";
  error_message?: string;
  created_at: string;
}

export interface R2Object {
  key: string;
  size: number;
  last_modified: string;
}

export interface SqlResult {
  type: "select" | "exec";
  columns: string[];
  rows: Record<string, unknown>[];
  rows_affected: number;
}
