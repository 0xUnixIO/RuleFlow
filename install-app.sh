#!/bin/sh

set -eu

GITHUB_REPO="0xUnixIO/RuleFlow"
GITHUB_BRANCH="${RULEFLOW_BRANCH:-main}"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_BRANCH}"
INSTALL_DIR="${RULEFLOW_DIR:-$HOME/ruleflow}"

ENV_FILE="$INSTALL_DIR/.env"
COMPOSE_FILE="$INSTALL_DIR/docker-compose.app.yaml"

log() {
  printf '%s\n' "$1"
}

read_kv() {
  grep "^$1=" "$ENV_FILE" 2>/dev/null | tail -n 1 | cut -d= -f2-
}

ensure_kv() {
  key=$1
  value=$2

  if grep -q "^$key=" "$ENV_FILE" 2>/dev/null; then
    tmp_file=$(mktemp)
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

download_file() {
  url=$1
  dest=$2
  if [ -f "$dest" ]; then
    cp "$dest" "${dest}.bak"
  fi
  curl -fsSL "$url" -o "$dest" || { log "下载失败: $url"; exit 1; }
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "缺少依赖: $1"
    exit 1
  fi
}

require_cmd curl

if ! command -v docker >/dev/null 2>&1; then
  printf "\n未检测到 Docker，是否自动安装？[Y/n] "
  read -r choice </dev/tty || true
  case "${choice:-Y}" in
    y|Y|"")
      log "正在安装 Docker..."
      curl -fsSL https://get.docker.com | sh
      if command -v systemctl >/dev/null 2>&1; then
        systemctl enable --now docker || true
      fi
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
download_file "$RAW_BASE/deploy/docker-compose.app.yaml" "$COMPOSE_FILE"
download_file "$RAW_BASE/uninstall.sh" "$INSTALL_DIR/uninstall.sh"
chmod +x "$INSTALL_DIR/uninstall.sh"

touch "$ENV_FILE"

IS_FIRST_INSTALL=false
if [ -z "$(read_kv DATABASE_URL)" ]; then
  IS_FIRST_INSTALL=true
fi

# DATABASE_URL
DATABASE_URL=$(read_kv DATABASE_URL)
if [ -z "$DATABASE_URL" ]; then
  printf "\n请输入 PostgreSQL 连接串\n"
  printf "格式: postgresql://user:pass@host:5432/dbname?sslmode=disable\n"
  printf "> "
  read -r DATABASE_URL </dev/tty || true
  if [ -z "$DATABASE_URL" ]; then
    log "DATABASE_URL 不能为空，已退出。"
    exit 1
  fi
fi

# REDIS_ADDR
REDIS_ADDR=$(read_kv REDIS_ADDR)
if [ -z "$REDIS_ADDR" ]; then
  printf "\n请输入 Redis 地址（默认 127.0.0.1:6379）: "
  read -r input_redis </dev/tty || true
  REDIS_ADDR="${input_redis:-127.0.0.1:6379}"
fi

# REDIS_PASSWORD
REDIS_PASSWORD=$(read_kv REDIS_PASSWORD)
if [ -z "$REDIS_PASSWORD" ]; then
  printf "请输入 Redis 密码（无密码直接回车）: "
  read -r REDIS_PASSWORD </dev/tty || true
fi

# SURGE_MANAGED_CONFIG_BASE_URL
BASE_URL=$(read_kv SURGE_MANAGED_CONFIG_BASE_URL)
if [ -z "$BASE_URL" ]; then
  printf "请输入服务的公开访问地址（如 https://ruleflow.example.com，留空跳过）: "
  read -r BASE_URL </dev/tty || true
fi

# PORT
PORT=$(read_kv PORT)
if [ -z "$PORT" ]; then
  printf "请输入监听端口（默认 8080）: "
  read -r input_port </dev/tty || true
  PORT="${input_port:-8080}"
fi

ensure_kv DATABASE_URL "$DATABASE_URL"
ensure_kv REDIS_ADDR "$REDIS_ADDR"
ensure_kv REDIS_PASSWORD "$REDIS_PASSWORD"
ensure_kv PORT "$PORT"
ensure_kv SURGE_MANAGED_CONFIG_BASE_URL "${BASE_URL:-}"

log ""
log "拉取镜像并启动..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

log ""
log "========================================="
log "  RuleFlow 已启动"
log "========================================="
log "  监听端口: $PORT"
if [ -n "${BASE_URL:-}" ]; then
  log "  访问地址: $BASE_URL"
fi
if [ "$IS_FIRST_INSTALL" = "true" ]; then
  log "  首次安装：请访问 /setup 创建管理员账户"
fi
log "========================================="
log "  查看日志: docker compose -f $COMPOSE_FILE logs -f"
log "  停止服务: docker compose -f $COMPOSE_FILE down"
log "  更新版本: docker compose -f $COMPOSE_FILE pull && docker compose -f $COMPOSE_FILE up -d"
