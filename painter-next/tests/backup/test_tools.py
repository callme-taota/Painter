import importlib.util
import base64
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import types
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


schedule = load("backup_schedule", ROOT / "scripts/backup-schedule.py")
recovery = load("recover_backup", ROOT / "scripts/recover-backup.py")


class BackupToolTests(unittest.TestCase):
    def test_environment_literal_parsing_does_not_execute(self):
        values = recovery.parse_environment("DATABASE_URL='host=/tmp password=literal$()'\nAPP_ENCRYPTION_KEY='opaque#key'\n")
        self.assertEqual(values["DATABASE_URL"], "host=/tmp password=literal$()")
        self.assertEqual(values["APP_ENCRYPTION_KEY"], "opaque#key")
        self.assertEqual(recovery.parse_environment(recovery.env_content(values)), values)
        self.assertEqual(recovery.database_name("postgres://role:pass@db/painter?sslmode=verify-full", "painter_recovery_abc"),
                         "postgres://role:pass@db/painter_recovery_abc?sslmode=verify-full")
        self.assertTrue(recovery.database_name("host=/tmp dbname=live", "painter_recovery_abc").endswith("dbname=painter_recovery_abc"))

    def test_systemd_native_and_compose_units_do_not_embed_secrets(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "runtime.env"
            config.write_text("APP_ENCRYPTION_KEY='secret-test-value'\n")
            config.chmod(0o600)
            args = types.SimpleNamespace(mode="native", binary=sys.executable, env_file=str(config),
                    backup_root=str(root / "backups"), keep=14, postgres_bin="", calendar="*-*-* 03:30:00")
            service, timer = schedule.units(args)
            self.assertIn("backup-run", service)
            self.assertIn("--config-snapshot", service)
            self.assertIn("EnvironmentFile=", service)
            self.assertNotIn("secret-test-value", service)
            self.assertIn("Persistent=true", timer)
            self.assertIn("RandomizedDelaySec=10min", timer)
            (root / ".env").write_text(config.read_text())
            (root / ".env").chmod(0o600)
            (root / "compose.yaml").write_text("services: {}\n")
            args.mode = "compose"
            args.project_root = str(root)
            args.docker = "/usr/bin/docker"
            args.backup_root = "/var/backups/painter/automatic"
            service, _ = schedule.units(args)
            self.assertIn("--no-deps", service)
            self.assertIn("--volume", service)
            self.assertIn("recovery.env:ro", service)
            self.assertNotIn("secret-test-value", service)
            config.chmod(0o644)
            with self.assertRaises(ValueError):
                schedule.private_file(config)

    def fixture(self, directory):
        root = Path(directory)
        runtime = root / "runtime.env"
        original = {"DATABASE_URL": "postgres://role:original-password@db/live?sslmode=disable",
                    "PUBLIC_BASE_URL": "https://current.example", "LISTEN_ADDR": "192.168.1.20:18080",
                    "GITHUB_CALLBACK_URL": "https://current.example/auth/github/callback",
                    "ENFORCE_PUBLIC_HTTPS": "true", "TRUSTED_PROXY_CIDRS": "192.168.1.20/32",
                    "APP_ENCRYPTION_KEY": "current-key", "ANALYTICS_HMAC_KEY": "current-analytics"}
        recovery.write_private(runtime, recovery.env_content(original))
        backup = root / "backup"
        backup.mkdir()
        recovery.write_private(backup / "recovery.env", recovery.env_content({
            "DATABASE_URL": "postgres://obsolete:do-not-restore@old/old", "PUBLIC_BASE_URL": "https://old.example",
            "APP_ENCRYPTION_KEY": base64.b64encode(bytes([1]) * 32).decode(), "ANALYTICS_HMAC_KEY": base64.b64encode(bytes([2]) * 32).decode()}))
        binary = root / "painter-test"
        binary.write_text("""#!/usr/bin/env python3
import json, os, pathlib, sys
if sys.argv[1] == 'restore-stage':
    options = dict(zip(sys.argv[2::2], sys.argv[3::2]))
    pathlib.Path(options['--media']).mkdir()
    print(json.dumps({'verified': True, 'migrations_verified': True, 'schema_version': 23, 'activated': False, 'database': options['--database']}))
else:
    from http.server import HTTPServer, BaseHTTPRequestHandler
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200); self.end_headers(); self.wfile.write(b'ok')
        def log_message(self, *args): pass
    host,port=os.environ['LISTEN_ADDR'].rsplit(':',1)
    HTTPServer((host,int(port)),Handler).serve_forever()
""")
        binary.chmod(0o700)
        args = types.SimpleNamespace(binary=str(binary), runtime_env=str(runtime), backup=str(backup),
                    workspace=str(root / "recovery"), runtime_role="", migration_role="", service="painter.service",
                    confirm_stop_writes=True)
        return args, original, runtime

    def test_stage_activate_rollback_preserves_tunnel_transport_and_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            args, original, runtime = self.fixture(directory)
            with mock.patch("builtins.print"):
                recovery.stage(args)
            workspace = Path(args.workspace)
            candidate = recovery.parse_environment(recovery.private_read(workspace / "candidate.env"))
            self.assertEqual(candidate["APP_ENCRYPTION_KEY"], base64.b64encode(bytes([1]) * 32).decode())
            self.assertIn("original-password@db/painter_recovery_", candidate["DATABASE_URL"])
            for key in ("PUBLIC_BASE_URL", "LISTEN_ADDR", "GITHUB_CALLBACK_URL", "ENFORCE_PUBLIC_HTTPS", "TRUSTED_PROXY_CIDRS"):
                self.assertEqual(candidate[key], original[key])
            self.assertEqual(recovery.parse_environment(recovery.private_read(runtime)), original)
            for path in workspace.glob("*.env"):
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            calls = []
            def systemctl(*parts, **kwargs):
                calls.append(parts)
                return parts[:1] != ("is-active",)
            with mock.patch.object(recovery, "systemctl", side_effect=systemctl), mock.patch.object(recovery, "ready", return_value=True), mock.patch("builtins.print"):
                recovery.change(args)
                self.assertEqual(recovery.parse_environment(recovery.private_read(runtime)), candidate)
                recovery.change(args, rollback=True)
                self.assertEqual(recovery.parse_environment(recovery.private_read(runtime)), original)
            self.assertFalse(any("tunnel" in " ".join(parts) for parts in calls))
            self.assertTrue((workspace / "media").is_dir())
            receipt = json.loads(recovery.private_read(workspace / "recovery.json"))
            self.assertEqual(receipt["phase"], "rolled_back")
            self.assertFalse(receipt["tunnel_modified"])

    def test_recovery_rejects_changed_root_keys_and_symlink_ancestors(self):
        with tempfile.TemporaryDirectory() as directory:
            args, _, _ = self.fixture(directory)
            backup = Path(args.backup)
            recovery.write_private(backup / "automatic.json", json.dumps({
                "private_configuration_included": True, "configuration_sha256": "wrong"}))
            with self.assertRaises(ValueError):
                recovery.stage(args)
            self.assertFalse(Path(args.workspace).exists())
            (backup / "automatic.json").unlink()
            target = Path(directory) / "linked-parent"
            target.symlink_to(backup, target_is_directory=True)
            with self.assertRaises(ValueError):
                recovery.private_read(target / "recovery.env")

    def test_activation_health_failure_restores_original_configuration(self):
        with tempfile.TemporaryDirectory() as directory:
            args, original, runtime = self.fixture(directory)
            with mock.patch("builtins.print"):
                recovery.stage(args)
            calls = []
            def systemctl(*parts, **kwargs):
                calls.append(parts)
                return parts[:1] != ("is-active",)
            with mock.patch.object(recovery, "systemctl", side_effect=systemctl), mock.patch.object(recovery, "ready", return_value=False):
                with self.assertRaises(ValueError):
                    recovery.change(args)
            self.assertEqual(recovery.parse_environment(recovery.private_read(runtime)), original)
            self.assertEqual(calls.count(("start", "painter.service")), 2)
            changed = recovery.private_read(runtime) + "# edited\n"
            recovery.write_private(runtime, changed, replace=True)
            with self.assertRaises(ValueError):
                recovery.change(args)


if __name__ == "__main__":
    unittest.main()
