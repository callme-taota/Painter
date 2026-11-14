#!/usr/bin/env bash
# Small terminal entry point. All operations delegate to the same locked CLI.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
if (($#)); then exec "$root/scripts/painter" "$@"; fi
if [[ ! -t 0 ]]; then
 printf '%s\n' 'Usage: ./maintain.sh <doctor|status|logs|backup|backups|verify-backup|start|restart|update|stop> [options]' 'Run without arguments in a terminal for the maintenance menu.' >&2
 exit 1
fi
printf '%s\n' 'Painter 维护工具' '  1  检查部署健康' '  2  查看服务状态' '  3  查看最近日志' '  4  完整备份（数据库、媒体、私有配置）' '  5  查看本机备份' '  6  验证备份恢复（独立临时库）' '  7  启动已有服务' '  8  重启应用' '  9  从已更新的源码升级（先备份）' '  10 停止服务（保留数据）' '  0  退出'
printf '请选择 [0]: ' >&2
IFS= read -r choice || exit 1
case "${choice:-0}" in
 0) exit 0;;
 1) exec "$root/scripts/painter" doctor;;
 2) exec "$root/scripts/painter" status;;
 3) exec "$root/scripts/painter" logs --tail 100;;
 4) exec "$root/scripts/painter" backup;;
 5) exec "$root/scripts/painter" backups;;
 6) "$root/scripts/painter" backups; printf '要验证的备份目录名: ' >&2; IFS= read -r backup_name || exit 1; exec "$root/scripts/painter" verify-backup --backup "$backup_name";;
 7) exec "$root/scripts/painter" start;;
 8) exec "$root/scripts/painter" restart;;
 9) exec "$root/scripts/painter" update --build;;
 10) exec "$root/scripts/painter" stop;;
 *) printf '%s\n' '无效选项；未执行操作。' >&2; exit 1;;
esac
