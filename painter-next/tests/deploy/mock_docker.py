#!/usr/bin/env python3
"""Small, strict Docker double used by the deployment contract tests.

It records argument arrays rather than shell strings. It never contacts Docker,
starts a process, reads a real env file, or accesses paths outside the fixture.
"""

import json
import os
from pathlib import Path
import sys


def record(args):
    with open(os.environ["MOCK_DOCKER_LOG"], "a", encoding="utf-8") as handle:
        handle.write(json.dumps(args) + "\n")
    # Observe only this non-secret key in the temporary fixture so tests can
    # check that image configuration is committed after health verification.
    image = ""
    env_file = Path(os.environ["MOCK_PROJECT_ROOT"]) / ".env"
    if env_file.exists():
        for line in env_file.read_text(encoding="utf-8").splitlines():
            if line.startswith("PAINTER_IMAGE="):
                image = line.partition("=")[2].strip("\"'")
    with open(os.environ["MOCK_DOCKER_OBSERVATIONS"], "a", encoding="utf-8") as handle:
        handle.write(json.dumps({"args": args, "configured_image": image, "process_image": os.environ.get("PAINTER_IMAGE")}) + "\n")


def setting(name, default=""):
    return os.environ.get("MOCK_DOCKER_" + name, default)


def fail(message, code=1):
    print(message, file=sys.stderr)
    raise SystemExit(int(setting("FAIL_CODE", str(code))))


def compose_command(args):
    # Global compose arguments may appear before the verb. Skip their values.
    takes_value = {"-f", "--file", "-p", "--project-name", "--project-directory", "--env-file", "--profile", "--ansi", "--progress"}
    index = 1
    while index < len(args):
        arg = args[index]
        if arg in takes_value:
            index += 2
        elif arg.startswith("-"):
            index += 1
        else:
            return arg, args[index + 1:]
    fail("mock docker: compose command missing", 97)


def inspect(args):
    is_migrate = any("migrate" in value for value in args)
    is_db = any(value == "mock-db" for value in args)
    state = {
        "Status": "exited" if is_migrate else setting("DB_STATUS" if is_db else "APP_STATUS", "running"),
        "Running": not is_migrate,
        "ExitCode": int(setting("MIGRATE_EXIT", "0")) if is_migrate else 0,
        "Health": {"Status": setting("DB_HEALTH" if is_db else "HEALTH", "healthy")},
    }
    format_string = ""
    for index, arg in enumerate(args):
        if arg in ("-f", "--format") and index + 1 < len(args):
            format_string = args[index + 1]
        elif arg.startswith("--format="):
            format_string = arg.split("=", 1)[1]
    if not format_string:
        print(json.dumps([{"State": state}]))
    elif "json .State" in format_string:
        print(json.dumps(state))
    elif "Health" in format_string:
        print(state["Health"]["Status"])
    elif "ExitCode" in format_string and "Status" in format_string:
        print(state["Status"] + " " + str(state["ExitCode"]))
    elif "ExitCode" in format_string:
        print(state["ExitCode"])
    elif "Running" in format_string:
        print(str(state["Running"]).lower())
    elif "Status" in format_string:
        print(state["Status"])
    else:
        fail("mock docker: unsupported inspect format " + format_string, 97)


def export_backup(args):
    if setting("CP_FAIL") == "1":
        fail("mock export failed")
    destination = Path(args[-1]).resolve()
    fixture = Path(os.environ["MOCK_PROJECT_ROOT"]).resolve()
    if not destination.is_relative_to(fixture):
        fail("mock docker: refused export outside temporary project", 97)
    source = args[-2]
    if destination.is_dir() and not source.endswith("/."):
        destination = destination / Path(source).name
    destination.mkdir(parents=True, exist_ok=True)
    (destination / "manifest.json").write_text('{"mock":true}\n', encoding="utf-8")
    (destination / "database.dump").write_bytes(b"mock PostgreSQL archive\n")


def main():
    args = sys.argv[1:]
    record(args)
    if not args:
        fail("mock docker: command missing", 97)
    if args[0] in ("version", "info"):
        if setting("DAEMON", "1") == "0":
            fail("Cannot connect to the Docker daemon")
        print("28.0.0" if "--format" in args else "Docker mock server 28.0.0")
    elif args[0] == "inspect":
        inspect(args[1:])
    elif args[0] == "cp":
        export_backup(args[1:])
    elif args[0] == "ps":
        if setting("EXISTING_PROJECT") == "1":
            print("mock-existing-project-container")
    elif args[0] == "volume" and len(args) > 1 and args[1] == "ls":
        if setting("EXISTING_VOLUME") == "1":
            print("mock-existing-project-volume")
    elif args[0] == "compose":
        command, rest = compose_command(args)
        if command == "version":
            if setting("COMPOSE", "1") == "0":
                fail("docker: compose is not a docker command")
            print("2.39.0")
        elif command == "config":
            if setting("CONFIG_FAIL") == "1":
                fail("mock invalid compose configuration")
        elif command == "ps":
            if setting("PS_FAIL") == "1":
                fail("mock service inspection failed")
            services = [value for value in rest if value in ("app", "migrate", "db", "caddy")]
            if "--format" in rest and "json" in rest:
                print(json.dumps({"Service": "app", "State": "running", "Health": setting("HEALTH", "healthy")}))
            elif any(flag in rest for flag in ("-q", "--quiet")):
                for service in services or ["app", "migrate"]:
                    print("mock-" + service)
            else:
                print("NAME          SERVICE   STATUS\nmock-app      app       Up (healthy)")
        elif command == "exec":
            if rest == ["-T", "app", "painter", "healthcheck"]:
                if setting("READINESS_FAIL") == "1":
                    fail("mock readiness failed")
                return
            if rest == ["-T", "app", "painter", "setup-token"]:
                if setting("SETUP_TOKEN_FAIL") == "1":
                    fail("mock installation already completed")
                print("mock-operator-installation-code")
                return
            if "backup" not in rest:
                fail("mock docker: unexpected exec " + repr(rest), 97)
            if setting("BACKUP_FAIL") == "1":
                fail("mock backup failed")
            print('{"id":"mock-backup","database_bytes":24}')
        elif command == "cp":
            export_backup(rest)
        elif command == "run":
            if len(rest) != 6 or rest[:5] != ["--rm", "--no-deps", "-T", "ops", "restore-check"]:
                fail("mock docker: unexpected run " + repr(rest), 97)
            if setting("RESTORE_FAIL") == "1":
                fail("mock restore check failed")
            print('{"verified":true,"database_dropped":true}')
        elif command in ("build", "pull", "up", "stop", "logs", "start", "restart"):
            if setting(command.upper() + "_FAIL") == "1":
                fail("mock " + command + " failed")
            if command == "logs":
                print("mock application log")
        else:
            fail("mock docker: unexpected compose command " + command, 97)
    else:
        fail("mock docker: unexpected command " + repr(args), 97)


if __name__ == "__main__":
    main()
