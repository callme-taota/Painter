#!/usr/bin/env bash
set -euo pipefail
DEFAULT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$DEFAULT_ROOT/scripts/lib/deploy.sh"
if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then printf '%s\n' 'Usage: scripts/configure.sh [--local | --lan-ip IPv4 | --domain hostname] [--port N] [--image tagged/image] [--non-interactive] [--root DIR]' 'Existing .env is preserved. Use scripts/painter install for Docker preflight and startup.'; exit 0; fi
parse_options "$@"
lock_operation
if [[ -e "$ROOT/.env" || -L "$ROOT/.env" ]]; then load_configuration; say 'Existing .env preserved; no secrets or configuration were replaced.'; else new_configuration; load_configuration; fi
say "Access URL: $URL"
