#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="$ROOT_DIR/.runtime"
LOG_DIR="$RUNTIME_DIR/logs"
PID_DIR="$RUNTIME_DIR/pids"

mkdir -p "$LOG_DIR" "$PID_DIR"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[dev-up] Missing command: $1"
    exit 1
  fi
}

start_container() {
  local name="$1"
  shift
  if docker ps --format '{{.Names}}' | grep -E "^${name}$" >/dev/null 2>&1; then
    echo "[dev-up] Container ${name} already running"
    return
  fi
  if docker ps -a --format '{{.Names}}' | grep -E "^${name}$" >/dev/null 2>&1; then
    echo "[dev-up] Starting existing container ${name}"
    docker start "$name" >/dev/null
    return
  fi
  echo "[dev-up] Creating container ${name}"
  docker run -d --name "$name" "$@" >/dev/null
}

start_proc() {
  local name="$1"
  shift
  local pid_file="$PID_DIR/${name}.pid"
  local log_file="$LOG_DIR/${name}.log"
  if [[ -f "$pid_file" ]]; then
    local old_pid
    old_pid="$(cat "$pid_file")"
    if kill -0 "$old_pid" >/dev/null 2>&1; then
      echo "[dev-up] ${name} already running (pid ${old_pid})"
      return
    fi
  fi
  echo "[dev-up] Starting ${name}"
  nohup "$@" >"$log_file" 2>&1 &
  echo $! >"$pid_file"
}

require_cmd docker
require_cmd go
require_cmd npm

if [[ "${USE_EXISTING_INFRA:-0}" != "1" ]]; then
  if ! docker info >/dev/null 2>&1; then
    echo "[dev-up] Docker daemon is not running."
    echo "[dev-up] Start Docker Desktop first, or run with USE_EXISTING_INFRA=1 if MySQL/Redis already exist."
    exit 1
  fi
  start_container painter-mysql -e MYSQL_ROOT_PASSWORD=root -p 3306:3306 mysql:8.0
  start_container painter-redis -p 6379:6379 redis:7
fi

if [[ "${USE_EXISTING_INFRA:-0}" != "1" ]]; then
  echo "[dev-up] Ensuring MySQL databases exist"
  docker exec -i painter-mysql mysql -uroot -proot -e "
CREATE DATABASE IF NOT EXISTS painter_identity CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS painter_content CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS painter_system CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS painter_analytics CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
" >/dev/null
fi

export IDENTITY_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/painter_identity?charset=utf8mb4&parseTime=True&loc=Local'
export CONTENT_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/painter_content?charset=utf8mb4&parseTime=True&loc=Local'
export SYSTEM_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/painter_system?charset=utf8mb4&parseTime=True&loc=Local'
export ANALYTICS_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/painter_analytics?charset=utf8mb4&parseTime=True&loc=Local'
export IDENTITY_REDIS_ADDR='127.0.0.1:6379'
export CONTENT_REDIS_ADDR='127.0.0.1:6379'
export SYSTEM_REDIS_ADDR='127.0.0.1:6379'
export IDENTITY_BASE_URL='http://localhost:18081'
export CONTENT_BASE_URL='http://localhost:18082'
export SYSTEM_BASE_URL='http://localhost:18083'
export ANALYTICS_BASE_URL='http://localhost:18084'

if [[ ! -d "$ROOT_DIR/node_modules" ]]; then
  echo "[dev-up] Installing npm dependencies"
  (cd "$ROOT_DIR" && npm install)
fi

start_proc identity-service env \
  IDENTITY_MYSQL_DSN="$IDENTITY_MYSQL_DSN" \
  IDENTITY_REDIS_ADDR="$IDENTITY_REDIS_ADDR" \
  IDENTITY_BASE_URL="$IDENTITY_BASE_URL" \
  bash -lc "cd '$ROOT_DIR' && go run ./services/identity-service/cmd/identity"

start_proc content-service env \
  CONTENT_MYSQL_DSN="$CONTENT_MYSQL_DSN" \
  CONTENT_REDIS_ADDR="$CONTENT_REDIS_ADDR" \
  CONTENT_BASE_URL="$CONTENT_BASE_URL" \
  bash -lc "cd '$ROOT_DIR' && go run ./services/content-service/cmd/content"

start_proc system-service env \
  SYSTEM_MYSQL_DSN="$SYSTEM_MYSQL_DSN" \
  SYSTEM_REDIS_ADDR="$SYSTEM_REDIS_ADDR" \
  SYSTEM_BASE_URL="$SYSTEM_BASE_URL" \
  bash -lc "cd '$ROOT_DIR' && go run ./services/system-service/cmd/system"

start_proc analytics-service env \
  ANALYTICS_MYSQL_DSN="$ANALYTICS_MYSQL_DSN" \
  ANALYTICS_BASE_URL="$ANALYTICS_BASE_URL" \
  bash -lc "cd '$ROOT_DIR' && go run ./services/analytics-service/cmd/analytics"

start_proc api-gateway env \
  IDENTITY_BASE_URL="$IDENTITY_BASE_URL" \
  CONTENT_BASE_URL="$CONTENT_BASE_URL" \
  SYSTEM_BASE_URL="$SYSTEM_BASE_URL" \
  ANALYTICS_BASE_URL="$ANALYTICS_BASE_URL" \
  bash -lc "cd '$ROOT_DIR' && go run ./services/api-gateway/cmd/gateway"

start_proc web-frontend bash -lc "cd '$ROOT_DIR' && npm run dev:web"
start_proc admin-frontend bash -lc "cd '$ROOT_DIR' && npm run dev:admin"

echo "[dev-up] All services started."
echo "[dev-up] Logs: $LOG_DIR"
echo "[dev-up] Web:   http://localhost:5173"
echo "[dev-up] Admin: http://localhost:5174"
echo "[dev-up] API:   http://localhost:18080"
