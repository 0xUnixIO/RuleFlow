#!/bin/sh

set -eu

GITHUB_REPO="0xUnixIO/RuleFlow"
GITHUB_BRANCH="${RULEFLOW_BRANCH:-main}"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_BRANCH}"
INSTALL_DIR="${RULEFLOW_DIR:-$HOME/ruleflow}"

ENV_FILE="$INSTALL_DIR/.env"
COMPOSE_FILE="$INSTALL_DIR/docker-compose.yaml"
CADDYFILE="$INSTALL_DIR/Caddyfile"

log() {
  printf '%s\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "缺少依赖: $1"
    exit 1
  fi
}

# 读取 .env 中某个 key 的值，正确处理值中含 = 的情况
read_kv() {
  grep "^$1=" "$ENV_FILE" 2>/dev/null | tail -n 1 | cut -d= -f2-
}

ensure_kv() {
  key=$1
  value=$2

  if grep -q "^$key=" "$ENV_FILE" 2>/dev/null; then
    tmp_file=$(mktemp)
    # 用 python-style 替换：只替换首个 = 前匹配的 key
    awk -v key="$key" -v value="$value" '
      BEGIN { updated = 0 }
      index($0, key "=") == 1 {
        print key "=" value
        updated = 1
        next
      }
      { print }
      END { if (updated == 0) print key "=" value }
    ' "$ENV_FILE" >"$tmp_file"
    mv "$tmp_file" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

gen_password() {
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24
}

download_file() {
  url=$1
  dest=$2
  if [ -f "$dest" ]; then
    cp "$dest" "${dest}.bak"
  fi
  curl -fsSL "$url" -o "$dest" || { log "下载失败: $url"; exit 1; }
}

require_cmd curl

# 检测并安装 Docker
if ! command -v docker >/dev/null 2>&1; then
  printf "\n未检测到 Docker，是否自动安装？[Y/n] "
  read -r choice </dev/tty || true
  case "${choice:-Y}" in
    y|Y|"")
      log "正在安装 Docker..."
      curl -fsSL https://get.docker.com | sh
      # 安装完成后启动 daemon
      if command -v systemctl >/dev/null 2>&1; then
        systemctl enable --now docker || true
      fi
      # 将当前用户加入 docker 组
      target_user="${SUDO_USER:-$(id -un)}"
      usermod -aG docker "$target_user" 2>/dev/null || true
      log "已将 $target_user 加入 docker 组，重新登录后可免 sudo 运行 docker"
      ;;
    *)
      log "已取消。请手动安装 Docker 后重试: https://docs.docker.com/engine/install/"
      exit 1
      ;;
  esac
fi

if ! docker info >/dev/null 2>&1; then
  log "Docker 未运行，请先启动 Docker 后重试。"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  log "需要 Docker Compose 插件（v2），请升级 Docker: https://docs.docker.com/engine/install/"
  exit 1
fi

log "创建安装目录: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

log "下载配置文件..."
download_file "$RAW_BASE/deploy/docker-compose.yaml" "$COMPOSE_FILE"
download_file "$RAW_BASE/deploy/Caddyfile" "$CADDYFILE"
download_file "$RAW_BASE/uninstall.sh" "$INSTALL_DIR/uninstall.sh"
chmod +x "$INSTALL_DIR/uninstall.sh"

# 初始化 .env（保留已有值）
touch "$ENV_FILE"

PG_PASSWORD=$(read_kv POSTGRES_PASSWORD)
PG_PASSWORD=${PG_PASSWORD:-$(gen_password)}

REDIS_PASSWORD=$(read_kv REDIS_PASSWORD)
REDIS_PASSWORD=${REDIS_PASSWORD:-$(gen_password)}

IS_FIRST_INSTALL=false
if [ -z "$(read_kv POSTGRES_PASSWORD)" ]; then
  IS_FIRST_INSTALL=true
fi

DOMAIN=$(read_kv DOMAIN)
if [ -z "$DOMAIN" ]; then
  printf "\n请输入域名（留空则使用 HTTP :80）: "
  read -r input_domain </dev/tty || true
  DOMAIN="${input_domain:-:80}"
fi

if [ "$DOMAIN" = ":80" ]; then
  HOST_IP=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -n1)
  [ -z "${HOST_IP:-}" ] && HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
  [ -z "${HOST_IP:-}" ] && HOST_IP=localhost
  BASE_URL="http://$HOST_IP"
else
  BASE_URL="https://$DOMAIN"
  # 检查域名 DNS 解析是否指向本机
  HOST_IP=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -n1)
  [ -z "${HOST_IP:-}" ] && HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
  if [ -n "${HOST_IP:-}" ]; then
    RESOLVED_IP=$(getent hosts "$DOMAIN" 2>/dev/null | awk '{print $1}' | head -n1)
    if [ -z "$RESOLVED_IP" ]; then
      log "⚠️  警告：域名 $DOMAIN 无法解析，请确认 DNS 记录已生效。"
      log "   Caddy 将无法自动签发 HTTPS 证书，服务可能无法访问。"
    elif [ "$RESOLVED_IP" != "$HOST_IP" ]; then
      log "⚠️  警告：域名 $DOMAIN 解析到 $RESOLVED_IP，但本机 IP 为 $HOST_IP。"
      log "   请检查 DNS A 记录是否正确指向本机。"
    else
      log "✓  DNS 解析正常：$DOMAIN → $RESOLVED_IP"
    fi
  fi
fi

POSTGRES_DB=$(read_kv POSTGRES_DB)
POSTGRES_USER=$(read_kv POSTGRES_USER)
ensure_kv POSTGRES_DB "${POSTGRES_DB:-ruleflow}"
ensure_kv POSTGRES_USER "${POSTGRES_USER:-ruleflow}"
ensure_kv POSTGRES_PASSWORD "$PG_PASSWORD"
ensure_kv REDIS_PASSWORD "$REDIS_PASSWORD"
ensure_kv DOMAIN "$DOMAIN"
ensure_kv PUBLIC_BASE_URL "$BASE_URL"
ensure_kv SURGE_MANAGED_CONFIG_BASE_URL "$BASE_URL"

log ""
log "拉取镜像并启动..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

log ""
log "========================================="
log "  RuleFlow 已启动"
log "========================================="
log "  访问地址: $BASE_URL"
if [ "$IS_FIRST_INSTALL" = "true" ]; then
  log "  首次安装：请访问 $BASE_URL/setup 创建管理员账户"
fi
log "========================================="
log "  查看日志: docker compose -f $COMPOSE_FILE logs -f"
log "  停止服务: docker compose -f $COMPOSE_FILE down"
log "  更新版本: docker compose -f $COMPOSE_FILE pull && docker compose -f $COMPOSE_FILE up -d"
