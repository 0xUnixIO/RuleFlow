#!/bin/sh

set -eu

if [ -n "${RULEFLOW_DIR:-}" ]; then
  ROOT_DIR="$RULEFLOW_DIR"
elif [ "$0" != "sh" ] && [ "$0" != "-" ] && [ "$0" != "/bin/sh" ]; then
  ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
else
  ROOT_DIR="${HOME}/ruleflow"
fi
ENV_FILE="$ROOT_DIR/.env"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yaml"

log() {
  printf '%s\n' "$1"
}

DOCKER_AVAILABLE=true
if ! command -v docker >/dev/null 2>&1; then
  DOCKER_AVAILABLE=false
elif ! docker info >/dev/null 2>&1; then
  DOCKER_AVAILABLE=false
fi

if [ ! -f "$COMPOSE_FILE" ] && [ "$DOCKER_AVAILABLE" = "false" ]; then
  log "未找到安装目录: $ROOT_DIR"
  log "如安装在其他位置，请设置 RULEFLOW_DIR 后重试。"
  exit 1
fi

printf "警告：此操作将永久删除所有数据（包括数据库）。确认卸载？[y/N] "
read -r confirm </dev/tty || true
case "${confirm:-N}" in
  y|Y) ;;
  *) log "已取消"; exit 0 ;;
esac

if [ "$DOCKER_AVAILABLE" = "true" ] && [ -f "$COMPOSE_FILE" ]; then
  log "停止并删除容器、网络和数据卷..."
  if [ -f "$ENV_FILE" ]; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down -v --remove-orphans
  else
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans
  fi
else
  log "Docker 不可用，跳过容器清理，仅删除文件。"
fi

log "删除安装目录: $ROOT_DIR"
rm -rf "$ROOT_DIR"

log "RuleFlow 已卸载"
