#!/usr/bin/env python3
"""Install a user systemd timer for Painter's existing backup/restore primitives.

No credentials are evaluated as shell code or placed in process arguments.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


def private_file(value):
    path = Path(value).absolute()
    info = path.lstat()
    if path.is_symlink() or not path.is_file() or info.st_mode & 0o077:
        raise ValueError("configuration must be an owner-only regular file (0600 or stricter)")
    if any(c in str(path) for c in "\n\r\x00"):
        raise ValueError("configuration path must be one line")
    return path


def quote(value):
    value = str(value)
    if any(c in value for c in "\n\r\x00"):
        raise ValueError("unit values must be one line")
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%') + '"'


def units(args):
    if args.mode == "native":
        binary = Path(args.binary).absolute()
        if not binary.is_file() or not os.access(binary, os.X_OK):
            raise ValueError("--binary must be an executable Painter binary")
        environment = private_file(args.env_file)
        command = [str(binary), "backup-run", "--root", str(Path(args.backup_root).absolute()),
                   "--keep", str(args.keep), "--config-snapshot", str(environment)]
        configuration = "EnvironmentFile=" + quote(environment) + "\n"
        if args.postgres_bin:
            directory = Path(args.postgres_bin).absolute()
            for name in ("pg_dump", "pg_restore"):
                if not (directory / name).is_file():
                    raise ValueError("--postgres-bin must contain pg_dump and pg_restore")
            configuration += "Environment=" + quote("POSTGRES_BIN_DIR=" + str(directory)) + "\n"
    else:
        root = Path(args.project_root).absolute()
        environment = private_file(root / ".env")
        if not (root / "compose.yaml").is_file() or ":" in str(environment):
            raise ValueError("--project-root must contain compose.yaml and a private .env")
        # Maintenance container is short-lived and deliberately gets the local
        # config only as a read-only bind mount. It is never passed via argv.
        command = [args.docker, "compose", "--project-directory", str(root), "--env-file", str(environment),
                   "-f", str(root / "compose.yaml"), "run", "--rm", "--no-deps", "-T", "--user", "0:0",
                   "--volume", str(environment) + ":/run/painter-recovery.env:ro", "ops", "backup-run",
                   "--root", args.backup_root, "--keep", str(args.keep),
                   "--config-snapshot", "/run/painter-recovery.env"]
        configuration = "WorkingDirectory=" + quote(root) + "\n"
        configuration += "UnsetEnvironment=DB_ADMIN_PASSWORD DB_MIGRATION_PASSWORD DB_APP_PASSWORD APP_ENCRYPTION_KEY ANALYTICS_HMAC_KEY PUBLIC_BASE_URL PUBLIC_HOST APP_PORT APP_BIND_ADDRESS PAINTER_IMAGE COMPOSE_PROJECT_NAME COMPOSE_PROFILES GITHUB_CLIENT_ID GITHUB_CLIENT_SECRET BOOTSTRAP_GITHUB_ID SETUP_TOKEN\n"
    service = ("[Unit]\nDescription=Painter verified automatic database and media backup\n"
               "After=network.target\n\n[Service]\nType=oneshot\nUMask=0077\n"
               "Nice=10\nIOSchedulingClass=idle\nNoNewPrivileges=true\n"
               "TimeoutStartSec=70min\n" + configuration +
               "ExecStart=" + " ".join(quote(value) for value in command) + "\n")
    timer = ("[Unit]\nDescription=Daily verified Painter backup\n\n[Timer]\n"
             "OnCalendar=" + args.calendar + "\nPersistent=true\nRandomizedDelaySec=10min\n"
             "AccuracySec=1min\nUnit=painter-backup.service\n\n[Install]\nWantedBy=timers.target\n")
    return service, timer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("install", "status", "disable"))
    parser.add_argument("--mode", choices=("native", "compose"), default="native")
    parser.add_argument("--binary", default=str(Path.home() / ".local/share/painter/bin/painter"))
    parser.add_argument("--env-file", default=str(Path.home() / ".config/painter/runtime-v33.env"))
    parser.add_argument("--backup-root", default="")
    parser.add_argument("--postgres-bin", default="")
    parser.add_argument("--project-root", default=str(Path(__file__).resolve().parent.parent))
    parser.add_argument("--docker", default="/usr/bin/docker")
    parser.add_argument("--keep", type=int, default=14)
    parser.add_argument("--calendar", default="*-*-* 03:30:00")
    parser.add_argument("--write-only", action="store_true", help="write units without activating the timer")
    args = parser.parse_args()
    if args.action == "status":
        return subprocess.run(["systemctl", "--user", "list-timers", "painter-backup.timer", "--all"], check=False).returncode
    if args.action == "disable":
        return subprocess.run(["systemctl", "--user", "disable", "--now", "painter-backup.timer"], check=False).returncode
    if not 1 <= args.keep <= 3650:
        raise ValueError("--keep must be 1..3650")
    if any(c in args.calendar for c in "\n\r\x00") or not args.calendar:
        raise ValueError("--calendar must be a one-line systemd calendar")
    subprocess.run(["systemd-analyze", "calendar", args.calendar], stdout=subprocess.DEVNULL,
                   stderr=subprocess.DEVNULL, check=True)
    if not args.backup_root:
        args.backup_root = (str(Path.home() / ".local/share/painter/backups/automatic")
                            if args.mode == "native" else "/var/backups/painter/automatic")
    service, timer = units(args)
    unit_dir = Path.home() / ".config/systemd/user"
    unit_dir.mkdir(parents=True, exist_ok=True)
    for name, content in (("painter-backup.service", service), ("painter-backup.timer", timer)):
        path = unit_dir / name
        if path.is_symlink():
            raise ValueError("refusing to overwrite a symlink unit")
        temp = unit_dir / (name + ".tmp-" + str(os.getpid()))
        with open(temp, "x", opener=lambda path, flags: os.open(path, flags, 0o600)) as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        temp.replace(path)
    if not args.write_only:
        subprocess.run(["systemctl", "--user", "daemon-reload"], check=True)
        subprocess.run(["systemctl", "--user", "enable", "--now", "painter-backup.timer"], check=True)
    print(json.dumps({"timer": "painter-backup.timer", "calendar": args.calendar,
                      "keep_verified": args.keep, "active": not args.write_only,
                      "mode": args.mode}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        # No configuration values or provider output are echoed.
        print("Painter backup scheduling failed: " + (str(error) if isinstance(error, ValueError)
              else "check file permissions and the user systemd manager"), file=sys.stderr)
        sys.exit(1)
