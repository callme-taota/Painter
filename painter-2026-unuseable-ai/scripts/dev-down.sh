#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_DIR="$ROOT_DIR/.runtime/pids"

stop_proc() {
  local name="$1"
  local pid_file="$PID_DIR/${name}.pid"
  if [[ ! -f "$pid_file" ]]; then
    echo "[dev-down] ${name} pid not found"
    return
  fi
  local pid
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    echo "[dev-down] Stopping ${name} (pid ${pid})"
    kill "$pid" >/dev/null 2>&1 || true
  else
    echo "[dev-down] ${name} already stopped"
  fi
  rm -f "$pid_file"
}

stop_proc web-frontend
stop_proc admin-frontend
stop_proc api-gateway
stop_proc analytics-service
stop_proc system-service
stop_proc content-service
stop_proc identity-service

echo "[dev-down] App processes stopped."
echo "[dev-down] To stop infra containers, run:"
echo "  docker stop painter-mysql painter-redis"
