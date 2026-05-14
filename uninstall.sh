#!/bin/sh

set -eu

DEFAULT_BOOTSTRAP_DIR=${RULEFLOW_DIR:-$HOME/ruleflow}

bootstrap_uninstall() {
  INSTALL_DIR=$DEFAULT_BOOTSTRAP_DIR

  if [ ! -d "$INSTALL_DIR" ]; then
    log "未找到安装目录: $INSTALL_DIR"
    log "如安装在其他位置，请先设置 RULEFLOW_DIR 后重试。"
    exit 1
  fi

  log "进入安装目录执行卸载: $INSTALL_DIR"
  exec sh "$INSTALL_DIR/uninstall.sh"
}

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE="$ROOT_DIR/.env"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yaml"
BIN_PATH="$ROOT_DIR/ruleflow"

log() {
  printf '%s\n' "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "缺少依赖: $1"
    exit 1
  fi
}

if [ "$(id -u)" -ne 0 ]; then
  log "请以 root 身份运行，例如: su -c 'sh uninstall.sh'"
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

if [ ! -f "$COMPOSE_FILE" ]; then
  bootstrap_uninstall
fi

# 停止 systemd 服务
if command -v systemctl >/dev/null 2>&1; then
  if systemctl is-active --quiet ruleflow 2>/dev/null; then
    log "停止 systemd 服务..."
    systemctl stop ruleflow
  fi
  if systemctl is-enabled --quiet ruleflow 2>/dev/null; then
    systemctl disable ruleflow
  fi
  if [ -f /etc/systemd/system/ruleflow.service ]; then
    rm -f /etc/systemd/system/ruleflow.service
    systemctl daemon-reload
    log "已删除 systemd 服务"
  fi
fi

log "停止并卸载 RuleFlow 容器、网络和数据卷..."
if [ -f "$ENV_FILE" ]; then
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down -v --remove-orphans
else
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans
fi

if [ -f "$ENV_FILE" ]; then
  rm -f "$ENV_FILE"
  log "已删除环境文件: $ENV_FILE"
fi

if [ -f "$BIN_PATH" ]; then
  rm -f "$BIN_PATH"
  log "已删除二进制: $BIN_PATH"
fi

if [ -d "$ROOT_DIR" ]; then
  rm -rf "$ROOT_DIR"
  log "已删除安装目录: $ROOT_DIR"
fi

log "RuleFlow 已卸载"
