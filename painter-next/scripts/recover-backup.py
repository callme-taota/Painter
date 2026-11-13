#!/usr/bin/env python3
"""Stage, validate, activate, or roll back a native user-systemd Painter recovery.

Production DB/media and Tunnel remain intact. Configuration values and child
provider output are never printed. Use the current deployment's binary and env.
"""
import argparse
import base64
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import secrets
import shlex
import socket
import subprocess
import sys
import time
import urllib.parse
import urllib.request

KEYS = ("APP_ENCRYPTION_KEY", "ENCRYPTION_KEY", "ANALYTICS_HMAC_KEY")


def reject_symlinks(path):
    path = Path(path).absolute()
    for component in (path, *path.parents):
        if component.is_symlink():
            raise ValueError("recovery paths must not contain symbolic links")


def private_read(path):
    path = Path(path).absolute()
    reject_symlinks(path)
    info = path.lstat()
    if path.is_symlink() or not path.is_file() or info.st_mode & 0o077 or info.st_size > 1 << 20:
        raise ValueError("expected an owner-only regular file (0600 or stricter, at most 1 MiB)")
    return path.read_text()


def parse_environment(content):
    result = {}
    for line in content.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        values = shlex.split(line, comments=True)
        if len(values) != 1 or "=" not in values[0]:
            raise ValueError("runtime env must contain literal KEY='value' assignments")
        key, value = values[0].split("=", 1)
        if not key or not key.replace("_", "A").isalnum() or key[0].isdigit():
            raise ValueError("invalid environment key")
        result[key] = value
    return result


def write_private(path, content, replace=False):
    path = Path(path)
    reject_symlinks(path)
    if path.is_symlink() or (path.exists() and not replace):
        raise ValueError("refusing to overwrite existing recovery file")
    temp = path.with_name(path.name + ".tmp-" + secrets.token_hex(8))
    try:
        with open(temp, "x", opener=lambda path, flags: os.open(path, flags, 0o600)) as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        if replace:
            os.replace(temp, path)
        else:
            os.link(temp, path)
            temp.unlink()
        fd = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        with contextlib.suppress(FileNotFoundError):
            temp.unlink()


def env_content(values):
    return "".join(key + "=" + shlex.quote(value) + "\n" for key, value in sorted(values.items()))


def database_name(dsn, name):
    if dsn.startswith(("postgres://", "postgresql://")):
        parts = urllib.parse.urlsplit(dsn)
        return urllib.parse.urlunsplit((parts.scheme, parts.netloc, "/" + name, parts.query, parts.fragment))
    return dsn + " dbname=" + name


def child_env(values):
    environment = os.environ.copy()
    # The supplied private file is authoritative. A stale caller DATABASE_URL,
    # proxy setting or encryption key must never silently override it.
    for key in ("DATABASE_URL", "MAINTENANCE_DATABASE_URL", "APP_ENCRYPTION_KEY", "ENCRYPTION_KEY",
                "ANALYTICS_HMAC_KEY", "PUBLIC_BASE_URL", "GITHUB_CALLBACK_URL", "LISTEN_ADDR",
                "ENFORCE_PUBLIC_HTTPS", "TRUSTED_PROXY_CIDRS", "RUN_BACKGROUND_WORKERS"):
        environment.pop(key, None)
    environment.update(values)
    return environment


def invoke(binary, arguments, values):
    result = subprocess.run([str(binary), *arguments], env=child_env(values), capture_output=True, text=True)
    if result.returncode:
        raise ValueError("Painter operation failed; private child/provider output withheld")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise ValueError("Painter returned an invalid recovery report") from error


def ready(address, deadline, process=None):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    while time.monotonic() < deadline:
        if process is not None and process.poll() is not None:
            return False
        try:
            with opener.open("http://" + address + "/readyz", timeout=1) as response:
                if response.status == 200:
                    return True
        except (OSError, ValueError):
            pass
        time.sleep(0.2)
    return False


def probe(binary, values):
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    candidate = values.copy()
    candidate["LISTEN_ADDR"] = "127.0.0.1:" + str(port)
    candidate["RUN_BACKGROUND_WORKERS"] = "false"
    process = subprocess.Popen([str(binary), "serve"], env=child_env(candidate),
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        if not ready(candidate["LISTEN_ADDR"], time.monotonic() + 30, process):
            raise ValueError("isolated candidate did not become healthy; active runtime was unchanged")
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


def systemctl(*arguments, check=True):
    result = subprocess.run(["systemctl", "--user", *arguments], stdout=subprocess.DEVNULL,
                            stderr=subprocess.DEVNULL)
    if check and result.returncode:
        raise ValueError("user systemd operation failed")
    return result.returncode == 0


def runtime_address(values):
    address = values.get("LISTEN_ADDR", ":8080")
    if address.startswith(":"):
        return "127.0.0.1" + address
    if address.startswith("0.0.0.0:"):
        return "127.0.0.1:" + address.rsplit(":", 1)[1]
    if address.startswith("[::]:"):
        return "[::1]:" + address.rsplit(":", 1)[1]
    return address


def sha(content):
    return hashlib.sha256(content.encode()).hexdigest()


def stage(args):
    binary = Path(args.binary).absolute()
    runtime = Path(args.runtime_env).absolute()
    current_text = private_read(runtime)
    current = parse_environment(current_text)
    backup = Path(args.backup).absolute()
    recovered_text = private_read(backup / "recovery.env")
    recovered_keys = parse_environment(recovered_text)
    if (backup / "automatic.json").exists():
        metadata = json.loads(private_read(backup / "automatic.json"))
        if not metadata.get("private_configuration_included") or metadata.get("configuration_sha256") != hashlib.sha256((backup / "recovery.env").read_bytes()).hexdigest():
            raise ValueError("automatic backup configuration checksum differs from its verified snapshot")
    if not current.get("DATABASE_URL") or not any(recovered_keys.get(key) for key in KEYS[:2]) or not recovered_keys.get("ANALYTICS_HMAC_KEY"):
        raise ValueError("backup must include its original encryption/analytics keys in recovery.env")
    encryption_key = recovered_keys.get("APP_ENCRYPTION_KEY") or recovered_keys.get("ENCRYPTION_KEY")
    try:
        encryption = base64.b64decode(encryption_key, validate=True)
        analytics = base64.b64decode(recovered_keys["ANALYTICS_HMAC_KEY"], validate=True)
    except (ValueError, TypeError) as error:
        raise ValueError("backup root keys must be valid base64 encoded 32 byte keys") from error
    if len(encryption) != 32 or len(analytics) != 32 or encryption == analytics:
        raise ValueError("backup requires independent 32 byte encryption and analytics keys")
    workspace = Path(args.workspace).absolute()
    reject_symlinks(workspace)
    workspace.mkdir(mode=0o700, parents=True, exist_ok=False)
    identifier = secrets.token_hex(10)
    database = "painter_recovery_" + identifier
    media = workspace / "media"
    arguments = ["restore-stage", "--source", str(backup), "--database", database, "--media", str(media)]
    for option, value in (("--runtime-role", args.runtime_role), ("--migration-role", args.migration_role)):
        if value:
            arguments.extend((option, value))
    operation_values = current.copy()
    postgres_bin = getattr(args, "postgres_bin", "")
    if not postgres_bin:
        bundled = Path.home() / ".local/share/painter/postgres18/distribution/usr/lib/postgresql/18/bin"
        if (bundled / "pg_dump").is_file() and (bundled / "pg_restore").is_file():
            postgres_bin = str(bundled)
    if postgres_bin:
        operation_values["POSTGRES_BIN_DIR"] = str(Path(postgres_bin).absolute())
    report = invoke(binary, arguments, operation_values)
    if (not report.get("verified") or not report.get("migrations_verified") or
            not isinstance(report.get("schema_version"), int) or report["schema_version"] < 1 or
            report.get("activated") or report.get("database") != database):
        raise ValueError("recovery stage did not return the expected isolated target")
    candidate = current.copy()
    for key in KEYS:
        candidate.pop(key, None)
        if key in recovered_keys:
            candidate[key] = recovered_keys[key]
    candidate["DATABASE_URL"] = database_name(current["DATABASE_URL"], database)
    if current.get("MAINTENANCE_DATABASE_URL"):
        candidate["MAINTENANCE_DATABASE_URL"] = database_name(current["MAINTENANCE_DATABASE_URL"], database)
    candidate["MEDIA_DIR"] = str(media)
    candidate_text = env_content(candidate)
    write_private(workspace / "previous.env", current_text)
    write_private(workspace / "candidate.env", candidate_text)
    # Probe always binds loopback and disables workers. It does not connect a
    # second public web endpoint, synchronize providers, or modify Tunnel routes.
    probe(binary, candidate)
    receipt = {"format": "painter-recovery-v1", "phase": "staged", "binary": str(binary),
               "runtime_env": str(runtime), "service": args.service,
               "original_sha256": sha(current_text), "candidate_sha256": sha(candidate_text),
               "database": database, "schema_version": report["schema_version"], "migrations_verified": True,
               "media_directory": str(media), "backup": str(backup),
               "isolated_health_verified": True, "activated": False,
               "transport_configuration_preserved": True, "tunnel_modified": False}
    write_private(workspace / "recovery.json", json.dumps(receipt, indent=2) + "\n")
    print(json.dumps({key: value for key, value in receipt.items() if key not in ("binary", "runtime_env")}, ensure_ascii=False))


def change(args, rollback=False):
    workspace = Path(args.workspace).absolute()
    receipt = json.loads(private_read(workspace / "recovery.json"))
    if receipt.get("format") != "painter-recovery-v1" or not receipt.get("isolated_health_verified"):
        raise ValueError("a verified recovery stage is required")
    runtime = Path(receipt["runtime_env"])
    original = private_read(workspace / "previous.env")
    candidate = private_read(workspace / "candidate.env")
    if sha(original) != receipt["original_sha256"] or sha(candidate) != receipt["candidate_sha256"]:
        raise ValueError("recovery configuration was changed after validation")
    old_text, new_text = (candidate, original) if rollback else (original, candidate)
    if private_read(runtime) != old_text:
        raise ValueError("active runtime changed after staging; stage again instead of overwriting it")
    service = receipt["service"]
    if service != "painter.service":
        raise ValueError("only the native painter.service unit is managed")
    if not args.confirm_stop_writes:
        raise ValueError("activation requires --confirm-stop-writes; content after the backup is not merged")
    lock_path = runtime.parent / "recovery-operation.lock"
    with open(lock_path, "a", opener=lambda path, flags: os.open(path, flags, 0o600)) as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ValueError("another recovery activation is running") from error
        if private_read(runtime) != old_text:
            raise ValueError("runtime configuration changed while waiting for recovery")
        timer_active = systemctl("is-active", "--quiet", "painter-backup.timer", check=False)
        if timer_active:
            systemctl("stop", "painter-backup.timer")
        # An existing oneshot must finish before DSN switching. Never cancel an
        # in-flight consistent backup/verification; ask the caller to retry later.
        if systemctl("is-active", "--quiet", "painter-backup.service", check=False):
            if timer_active:
                systemctl("start", "painter-backup.timer")
            raise ValueError("backup currently running; retry after it completes")
        switched = False
        try:
            systemctl("stop", service)
            write_private(runtime, new_text, replace=True)
            switched = True
            systemctl("start", service)
            if not ready(runtime_address(parse_environment(new_text)), time.monotonic() + 30):
                raise ValueError("activated runtime did not become healthy")
            receipt["phase"] = "rolled_back" if rollback else "active"
            receipt["activated"] = not rollback
            receipt["local_health_verified"] = True
            write_private(workspace / "recovery.json", json.dumps(receipt, indent=2) + "\n", replace=True)
        except BaseException:
            if switched:
                systemctl("stop", service, check=False)
                write_private(runtime, old_text, replace=True)
            systemctl("start", service, check=False)
            raise
        finally:
            if timer_active:
                systemctl("start", "painter-backup.timer", check=False)
    print(json.dumps({"phase": receipt["phase"], "activated": receipt["activated"],
                      "local_health_verified": True, "previous_database_retained": True,
                      "tunnel_modified": False}, ensure_ascii=False))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("stage", "activate", "rollback"))
    parser.add_argument("--workspace", required=True, help="new private recovery directory; reused for activation/rollback")
    parser.add_argument("--backup", default="")
    parser.add_argument("--runtime-env", default=str(Path.home() / ".config/painter/runtime-v33.env"))
    parser.add_argument("--binary", default=str(Path.home() / ".local/share/painter/bin/painter"))
    parser.add_argument("--service", choices=("painter.service",), default="painter.service")
    parser.add_argument("--runtime-role", default="")
    parser.add_argument("--migration-role", default="")
    parser.add_argument("--postgres-bin", default="", help="PostgreSQL 18 client directory; auto-detect the bundled local installation")
    parser.add_argument("--confirm-stop-writes", action="store_true")
    args = parser.parse_args()
    if args.action == "stage":
        if not args.backup:
            parser.error("stage requires --backup")
        stage(args)
    else:
        change(args, rollback=args.action == "rollback")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, json.JSONDecodeError):
        # Keep private env/DSNs and provider output out of terminal/journal logs.
        print("Painter recovery failed; inspect the private workspace and current service health. "
              "Existing databases, media, and Tunnel were retained.", file=sys.stderr)
        sys.exit(1)
