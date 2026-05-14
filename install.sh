#!/bin/sh

set -eu

GITHUB_REPO="ablate-ai/RuleFlow"
GITHUB_BRANCH="${RULEFLOW_BRANCH:-main}"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_BRANCH}"
INSTALL_DIR="${RULEFLOW_DIR:-$HOME/ruleflow}"

ENV_FILE="$INSTALL_DIR/.env"
COMPOSE_FILE="$INSTALL_DIR/deploy/docker-compose.yaml"
BIN_PATH="$INSTALL_DIR/ruleflow"

log() {
  printf '%s\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "缺少依赖: $1"
    exit 1
  fi
}

ensure_kv() {
  key=$1
  value=$2

  if grep -q "^$key=" "$ENV_FILE"; then
    tmp_file=$(mktemp)
    awk -F= -v key="$key" -v value="$value" '
      BEGIN { updated = 0 }
      $1 == key {
        print key "=" value
        updated = 1
        next
      }
      { print }
      END {
        if (updated == 0) {
          print key "=" value
        }
      }
    ' "$ENV_FILE" >"$tmp_file"
    mv "$tmp_file" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

port_in_use() {
  port=$1
  ss -tlnp 2>/dev/null | grep -q ":$port " || \
  netstat -tlnp 2>/dev/null | grep -q ":$port "
}

find_free_port() {
  while true; do
    p=$(awk 'BEGIN{srand(); print int(rand()*16383)+49152}')
    if ! port_in_use "$p"; then
      printf '%s' "$p"
      return
    fi
  done
}

check_port() {
  port=$1
  desc=$2
  allow_random=${3:-false}

  if ! port_in_use "$port"; then
    return
  fi

  while true; do
    printf "\n端口 %s (%s) 已被占用。\n" "$port" "$desc"
    if [ "$allow_random" = "true" ]; then
      printf "  [r] 随机换一个\n  [c] 继续使用\n  [q] 取消安装\n请选择: "
    else
      printf "  [c] 继续使用现有服务\n  [q] 取消安装\n请选择: "
    fi
    choice=""
    read -r choice </dev/tty || true
    case "$choice" in
      r|R)
        if [ "$allow_random" = "true" ]; then
          new_port=$(find_free_port)
          log "已切换到随机端口: $new_port"
          PORT_VALUE=$new_port
          return
        fi
        ;;
      c|C|"")
        log "继续使用端口 $port"
        return
        ;;
      q|Q)
        log "已取消安装。"
        exit 1
        ;;
    esac
  done
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64)        printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *)
      log "不支持的架构: $arch"
      exit 1
      ;;
  esac
}

download_files() {
  require_cmd curl

  log "创建安装目录: $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR/deploy" "$INSTALL_DIR/migrations"

  log "下载 docker-compose.yaml..."
  curl -fsSL "$RAW_BASE/deploy/docker-compose.yaml" -o "$COMPOSE_FILE"

  log "下载 migrations/init.sql..."
  curl -fsSL "$RAW_BASE/migrations/init.sql" -o "$INSTALL_DIR/migrations/init.sql"

  log "下载 uninstall.sh..."
  curl -fsSL "$RAW_BASE/uninstall.sh" -o "$INSTALL_DIR/uninstall.sh"
  chmod +x "$INSTALL_DIR/uninstall.sh"
}

download_binary() {
  require_cmd curl

  ARCH=$(detect_arch)
  BINARY_NAME="ruleflow-linux-${ARCH}"
  DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY_NAME}"

  log "下载二进制: $DOWNLOAD_URL"
  curl -fsSL "$DOWNLOAD_URL" -o "$BIN_PATH"
  chmod +x "$BIN_PATH"
  log "下载完成: $BIN_PATH"
}

install_systemd_service() {
  SERVICE_FILE="/etc/systemd/system/ruleflow.service"

  log "创建 systemd 服务: $SERVICE_FILE"
  cat >"$SERVICE_FILE" <<EOF
[Unit]
Description=RuleFlow
After=network.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=$BIN_PATH
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable ruleflow
  systemctl restart ruleflow
}

if [ "$(id -u)" -ne 0 ]; then
  log "请以 root 身份运行，例如: sudo curl ... | sh"
  exit 1
fi

require_cmd docker

if ! docker info >/dev/null 2>&1; then
  log "Docker 未运行，请先启动 Docker Desktop 或 Docker Engine。"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  log "当前 Docker 不支持 'docker compose'。请升级 Docker / Compose 插件。"
  exit 1
fi

# 检测已有安装（以 systemd 服务是否注册为准）
IS_REINSTALL=false
if systemctl list-unit-files ruleflow.service 2>/dev/null | grep -q ruleflow; then
  IS_REINSTALL=true
  if systemctl is-active --quiet ruleflow 2>/dev/null; then
    log "检测到 RuleFlow 已在运行，停止旧服务后继续..."
    systemctl stop ruleflow
  else
    log "检测到已有安装，执行更新..."
  fi
fi

PORT_VALUE=$(awk -F= '/^PORT=/{print $2}' "$ENV_FILE" 2>/dev/null | tail -n 1)
if [ -z "${PORT_VALUE:-}" ]; then
  PORT_VALUE=8080
fi

# 重装时基础设施已在运行，跳过端口检查
if [ "$IS_REINSTALL" = "false" ]; then
  check_port "$PORT_VALUE" "RuleFlow" true
  check_port 5432 "PostgreSQL"
  check_port 6379 "Redis"
fi

download_files

# 初始化 .env
if [ ! -f "$ENV_FILE" ]; then
  touch "$ENV_FILE"
fi

# 生成随机密码（仅首次安装时生成，重装保留原有值）
gen_password() {
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24
}

EXISTING_PG_PASS=$(awk -F= '/^POSTGRES_PASSWORD=/{print $2}' "$ENV_FILE" | tail -n 1)
if [ -z "${EXISTING_PG_PASS:-}" ]; then
  PG_PASSWORD=$(gen_password)
else
  PG_PASSWORD=$EXISTING_PG_PASS
fi

EXISTING_ADMIN_PASS=$(awk -F= '/^ADMIN_PASSWORD=/{print $2}' "$ENV_FILE" | tail -n 1)
if [ -z "${EXISTING_ADMIN_PASS:-}" ]; then
  ADMIN_PASSWORD=$(gen_password)
else
  ADMIN_PASSWORD=$EXISTING_ADMIN_PASS
fi

ensure_kv POSTGRES_DB ruleflow
ensure_kv POSTGRES_USER ruleflow
ensure_kv POSTGRES_PASSWORD "$PG_PASSWORD"
ensure_kv DATABASE_URL "postgresql://ruleflow:${PG_PASSWORD}@localhost:5432/ruleflow?sslmode=disable"
ensure_kv REDIS_ADDR 'localhost:6379'
ensure_kv PORT "$PORT_VALUE"
ensure_kv ADMIN_PASSWORD "$ADMIN_PASSWORD"

download_binary

log "启动基础设施（postgres + redis）..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d

log "启动 RuleFlow..."
install_systemd_service

# 获取本机 IP（优先取第一个非 loopback 地址）
HOST_IP=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -n1)
if [ -z "${HOST_IP:-}" ]; then
  HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
fi
if [ -z "${HOST_IP:-}" ]; then
  HOST_IP=localhost
fi

log ""
log "========================================="
log "  RuleFlow 已启动"
log "========================================="
log "  访问地址:   http://$HOST_IP:$PORT_VALUE"
log "  管理密码:   $ADMIN_PASSWORD"
log "  数据库密码: $PG_PASSWORD"
log "========================================="
log "  查看日志: journalctl -u ruleflow -f"
