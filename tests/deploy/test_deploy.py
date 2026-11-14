"""Deployment CLI contracts. Run: python3 -m unittest discover -s tests/deploy -v.

All invocations use a copied temporary project and a strict Docker double.
These tests do not require Docker and must not operate on a developer's stack.
"""

import base64
import errno
import hashlib
import json
import os
from pathlib import Path
import pty
import select
import shutil
import stat
import subprocess
import tarfile
import tempfile
import time
import unittest


PROJECT = Path(__file__).resolve().parents[2]
MOCK_DOCKER = Path(__file__).with_name("mock_docker.py")
SECRET_KEYS = (
    "DB_ADMIN_PASSWORD", "DB_MIGRATION_PASSWORD", "DB_APP_PASSWORD",
    "APP_ENCRYPTION_KEY", "ANALYTICS_HMAC_KEY", "SETUP_TOKEN",
)
IMAGE = "ghcr.io/example/painter-next@sha256:" + "a" * 64
NEW_IMAGE = "ghcr.io/example/painter-next@sha256:" + "b" * 64
BUNDLE_FILES = {
    "install.sh", "maintain.sh", "compose.yaml", "README.txt", "docs/DEPLOYMENT.md", "docs/deployment-tool.md", "docs/admin-cli.md", "docs/local-accounts.md",
    "docs/INSTALLATION_V7.md", "docs/GITHUB_LOGIN_V7.md", "docs/ACCOUNT_SECURITY.md",
    "docs/IDENTITY_PROVIDER.md", "docs/AUTOMATIC_BACKUP_RECOVERY.md",
    "scripts/painter", "scripts/configure.sh", "scripts/install-docker.sh",
    "scripts/package-deploy.sh", "scripts/backup-schedule.py", "scripts/recover-backup.py",
    "scripts/lib/deploy.sh", "deploy/Caddyfile",
    "deploy/init-roles.sh", "deploy/example.env", "deploy/images.lock.json",
    "deploy/release-image.txt",
}


class DeployTests(unittest.TestCase):
    maxDiff = None

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="painter-deploy-test-")
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / "project with spaces"
        self.root.mkdir()
        for name in ("scripts", "deploy"):
            shutil.copytree(PROJECT / name, self.root / name)
        shutil.copy2(PROJECT / "compose.yaml", self.root / "compose.yaml")
        shutil.copy2(PROJECT / "install.sh", self.root / "install.sh")
        shutil.copy2(PROJECT / "maintain.sh", self.root / "maintain.sh")
        # Copy the real build-eligibility files; Docker build remains mocked.
        for name in ("Dockerfile", "go.mod", "web/frontend/package-lock.json"):
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(PROJECT / name, target)
        self.bin = self.base / "bin"
        self.bin.mkdir()
        shutil.copy2(MOCK_DOCKER, self.bin / "docker")
        (self.bin / "docker").chmod(0o755)
        self.docker_log = self.base / "docker.jsonl"
        self.docker_observations = self.base / "observations.jsonl"
        self.external_log = self.base / "unexpected-external.txt"
        # Guard commands that could escape the test double and mutate a host.
        for command in ("git", "curl", "wget", "sudo", "systemctl"):
            (self.bin / command).write_text(
                "#!/bin/sh\nprintf '%s\\n' \"$0 $*\" >> \"$MOCK_EXTERNAL_LOG\"\nexit 97\n",
                encoding="utf-8",
            )
            (self.bin / command).chmod(0o755)
        self.env = dict(os.environ)
        self.env.update({
            "PATH": str(self.bin) + os.pathsep + os.environ.get("PATH", ""),
            "HOME": str(self.base / "home"),
            "LC_ALL": "C",
            "MOCK_DOCKER_LOG": str(self.docker_log),
            "MOCK_DOCKER_OBSERVATIONS": str(self.docker_observations),
            "MOCK_EXTERNAL_LOG": str(self.external_log),
            "MOCK_PROJECT_ROOT": str(self.root),
        })
        self.addCleanup(self.assert_no_external_mutation)

    def assert_no_external_mutation(self):
        if self.external_log.exists():
            self.fail("Deployment invoked an unexpected external command: " + self.external_log.read_text())

    def run_script(self, script, args, *, settings=None, expected=0, with_root=True):
        script_path = self.root / "scripts" / script
        self.assertTrue(script_path.is_file(), "Deployment entry point is missing: " + str(script_path))
        self.assertTrue(os.access(script_path, os.X_OK), "Deployment entry point is not executable: " + str(script_path))
        environment = self.env.copy()
        environment.update({"MOCK_DOCKER_" + key: str(value) for key, value in (settings or {}).items()})
        arguments = [*args, "--root", str(self.root)] if with_root else list(args)
        result = subprocess.run(
            [str(script_path), *arguments],
            cwd=self.base, env=environment, stdin=subprocess.DEVNULL,
            text=True, capture_output=True, timeout=12,
        )
        self.assertNotIn("mock docker:", result.stdout + result.stderr, "Test double contract mismatch: " + result.stdout + result.stderr)
        if expected == 0:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        elif expected == "failure":
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        elif expected is not None:
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        return result

    def configure(self, *args, **kwargs):
        return self.run_script("configure.sh", [*args, "--non-interactive"], **kwargs)

    def interactive_configure(self, args, dialogue):
        """Answer each real TTY prompt only after observing it."""
        master, slave = pty.openpty()
        process = None
        output = bytearray()
        deadline = time.monotonic() + 12

        def read_output():
            remaining = deadline - time.monotonic()
            self.assertGreater(remaining, 0, "Interactive configuration timed out: " + output.decode())
            ready, _, _ = select.select([master], [], [], remaining)
            self.assertTrue(ready, "Interactive configuration timed out: " + output.decode())
            try:
                chunk = os.read(master, 65536)
            except OSError as error:
                if error.errno != errno.EIO:
                    raise
                return False
            output.extend(chunk)
            return bool(chunk)

        try:
            process = subprocess.Popen(
                [str(self.root / "scripts/configure.sh"), *args, "--root", str(self.root)],
                cwd=self.base, env=self.env, stdin=slave, stdout=slave, stderr=slave,
            )
            os.close(slave)
            slave = None
            position = 0
            for prompt, answer in dialogue:
                expected = prompt.encode()
                while expected not in output[position:]:
                    self.assertTrue(read_output(), "Missing interactive prompt: " + prompt + "\n" + output.decode())
                position = output.index(expected, position) + len(expected)
                # In particular, LAN detection must wait for address confirmation.
                self.assertFalse((self.root / ".env").exists(), "Configuration was written before the wizard finished")
                os.write(master, (answer + "\n").encode())
            while read_output():
                pass
            process.wait(timeout=max(0.1, deadline - time.monotonic()))
            result = subprocess.CompletedProcess(process.args, process.returncode, output.decode(), "")
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertNotIn("mock docker:", result.stdout)
            return result
        finally:
            if process is not None and process.poll() is None:
                process.kill()
                process.wait()
            if slave is not None:
                os.close(slave)
            os.close(master)

    def painter(self, command, *args, **kwargs):
        return self.run_script("painter", [command, *args, "--non-interactive"], **kwargs)

    def package(self, output, *, image=IMAGE, expected=0):
        for name in ("DEPLOYMENT.md", "deployment-tool.md", "admin-cli.md", "local-accounts.md", "INSTALLATION_V7.md", "GITHUB_LOGIN_V7.md", "ACCOUNT_SECURITY.md", "IDENTITY_PROVIDER.md", "AUTOMATIC_BACKUP_RECOVERY.md"):
            target = self.root / "docs" / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(PROJECT / "docs" / name, target)
        return self.run_script("package-deploy.sh", ["--image", image, "--output", str(output)], expected=expected, with_root=False)

    def values(self):
        result = {}
        for line in (self.root / ".env").read_text(encoding="utf-8").splitlines():
            if line and not line.startswith("#"):
                key, value = line.split("=", 1)
                if len(value) >= 2 and value.startswith("'") and value.endswith("'"):
                    value = value[1:-1].replace("\\'", "'")
                elif len(value) >= 2 and value.startswith('"') and value.endswith('"'):
                    value = value[1:-1]
                result[key] = value
        return result

    def commands(self):
        if not self.docker_log.exists():
            return []
        return [json.loads(line) for line in self.docker_log.read_text().splitlines()]

    def observations(self):
        if not self.docker_observations.exists():
            return []
        return [json.loads(line) for line in self.docker_observations.read_text().splitlines()]

    @staticmethod
    def has_verb(command, verb):
        return "compose" in command and verb in command

    def assert_no_start(self):
        self.assertFalse(any(self.has_verb(command, verb) for command in self.commands() for verb in ("build", "up")), self.commands())

    def assert_no_stop(self):
        self.assertFalse(any(self.has_verb(command, "stop") for command in self.commands()), self.commands())

    def assert_migrations_before_app_activation(self):
        commands = self.commands()
        migrate_up = next(index for index, command in enumerate(commands) if self.has_verb(command, "up") and "migrate" in command)
        app_up = next(index for index, command in enumerate(commands) if self.has_verb(command, "up") and "app" in command)
        preparation = next(index for index, command in enumerate(commands) if self.has_verb(command, "build") or self.has_verb(command, "pull"))
        stop = next(index for index, command in enumerate(commands) if self.has_verb(command, "stop") and "app" in command)
        self.assertLess(preparation, stop)
        self.assertLess(stop, migrate_up)
        self.assertEqual(commands[stop][commands[stop].index("stop") + 1:], ["--timeout", "30", "app"])
        self.assertIn("--force-recreate", commands[migrate_up])
        self.assertIn("--no-build", commands[migrate_up])
        self.assertNotIn("app", commands[migrate_up])
        migration_exit_checks = [index for index, command in enumerate(commands) if command[0] == "inspect" and any("migrate" in arg for arg in command) and any("ExitCode" in arg for arg in command)]
        self.assertTrue(migration_exit_checks, commands)
        self.assertLess(migrate_up, migration_exit_checks[0])
        self.assertLess(migration_exit_checks[-1], app_up)
        health_checks = [index for index, command in enumerate(commands) if command[0] == "inspect" and any("Health" in arg for arg in command)]
        self.assertTrue(health_checks, commands)
        self.assertLess(app_up, health_checks[0])

    def assert_private_env(self, result):
        path = self.root / ".env"
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        values = self.values()
        secrets = [values[key] for key in SECRET_KEYS]
        self.assertTrue(all(len(secret) >= 32 for secret in secrets), "Generated secrets must have at least 32 characters")
        self.assertEqual(len(set(secrets)), len(SECRET_KEYS), "Generated secrets must be independent")
        for key in ("APP_ENCRYPTION_KEY", "ANALYTICS_HMAC_KEY"):
            self.assertEqual(len(base64.b64decode(values[key], validate=True)), 32, key + " must encode 32 bytes for ConfigFromEnv")
        for secret in secrets:
            self.assertNotIn(secret, result.stdout + result.stderr)
        return values

    def assert_local_account_guidance(self, result, url):
        output = result.stdout + result.stderr
        self.assertIn("Local account login: " + url + "/login", output)
        self.assertIn("First installation: " + url + "/setup", output)
        self.assertIn("./scripts/painter setup-token", output)
        self.assertIn("No default account or password is created", output)
        self.assertIn("offline painter create-local-user CLI", output)
        self.assertIn("docs/DEPLOYMENT.md", output)
        self.assertIn("Production registration is closed by default", output)
        self.assertIn("registration_enabled", output)
        self.assertIn("GitHub OAuth is optional. Callback: " + url + "/auth/github/callback", output)
        self.assertNotIn("Configure real GitHub OAuth to sign in", output)
        self.assertFalse(any("create-local-user" in command for command in self.commands()), self.commands())

    def test_configuration_defaults_to_local_and_private_independent_secrets(self):
        result = self.configure()
        values = self.assert_private_env(result)
        self.assertEqual(values["APP_BIND_ADDRESS"], "127.0.0.1")
        self.assertEqual(values["APP_PORT"], "8080")
        self.assertEqual(values["PUBLIC_BASE_URL"], "http://localhost:8080")
        self.assertEqual(stat.S_IMODE((self.root / "backups").stat().st_mode), 0o700)
        self.assertEqual(self.commands(), [])

    def test_non_tty_without_mode_still_defaults_to_local(self):
        self.run_script("configure.sh", [])
        values = self.values()
        self.assertEqual(values["PAINTER_DEPLOY_MODE"], "local")
        self.assertEqual(values["APP_BIND_ADDRESS"], "127.0.0.1")
        self.assertEqual(values["PUBLIC_BASE_URL"], "http://localhost:8080")
        self.assertEqual(self.commands(), [])

    def test_interactive_default_lan_requires_ipv4_confirmation(self):
        detector = self.bin / "ip"
        detector.write_text("#!/bin/sh\nprintf '%s\\n' '1.1.1.1 via 192.168.12.1 dev eth0 src 192.168.12.34'\n")
        detector.chmod(0o755)
        result = self.interactive_configure([], [
            ("Access mode: local, lan, https [lan]: ", ""),
            ("Private LAN IPv4 (bind only this address) [192.168.12.34]: ", ""),
            ("Host application port [8080]: ", ""),
            ("Configure optional GitHub OAuth now? y/N [N]: ", ""),
        ])
        values = self.assert_private_env(result)
        self.assertEqual(values["PAINTER_DEPLOY_MODE"], "lan")
        self.assertEqual(values["APP_BIND_ADDRESS"], "192.168.12.34")
        self.assertEqual(values["PUBLIC_BASE_URL"], "http://192.168.12.34:8080")
        self.assertEqual(self.commands(), [])

    def test_interactive_explicit_modes_override_lan_default(self):
        cases = (
            ((), [("Access mode: local, lan, https [lan]: ", "local")], "local", "127.0.0.1", "http://localhost:8080"),
            (("--local",), [], "local", "127.0.0.1", "http://localhost:8080"),
            (("--mode", "local"), [], "local", "127.0.0.1", "http://localhost:8080"),
            (("--mode", "lan", "--lan-ip", "192.168.12.35"), [], "lan", "192.168.12.35", "http://192.168.12.35:8080"),
            (("--domain", "blog.example.com"), [], "https", "127.0.0.1", "https://blog.example.com"),
            (("--mode", "https", "--domain", "blog.example.com"), [], "https", "127.0.0.1", "https://blog.example.com"),
        )
        for args, mode_dialogue, mode, bind, url in cases:
            with self.subTest(args=args):
                (self.root / ".env").unlink(missing_ok=True)
                result = self.interactive_configure(args, [*mode_dialogue,
                    ("Host application port [8080]: ", ""),
                    ("Configure optional GitHub OAuth now? y/N [N]: ", ""),
                ])
                values = self.values()
                self.assertEqual(values["PAINTER_DEPLOY_MODE"], mode)
                self.assertEqual(values["APP_BIND_ADDRESS"], bind)
                self.assertEqual(values["PUBLIC_BASE_URL"], url)
                self.assertNotIn("Private LAN IPv4", result.stdout)
                if args:
                    self.assertNotIn("Access mode:", result.stdout)
        self.assertEqual(self.commands(), [])

    def test_local_legacy_flag_and_custom_port(self):
        self.configure("--local", "--port", "18080")
        values = self.values()
        self.assertEqual(values["PUBLIC_BASE_URL"], "http://localhost:18080")
        self.assertEqual(values["APP_BIND_ADDRESS"], "127.0.0.1")
        self.assertEqual(values["APP_PORT"], "18080")

    def test_explicit_local_mode(self):
        self.configure("--mode", "local")
        self.assertEqual(self.values()["APP_BIND_ADDRESS"], "127.0.0.1")

    def test_lan_binds_only_the_supplied_ipv4(self):
        self.configure("--mode", "lan", "--lan-ip", "192.168.12.34", "--port", "18081")
        values = self.values()
        self.assertEqual(values["APP_BIND_ADDRESS"], "192.168.12.34")
        self.assertEqual(values["PUBLIC_BASE_URL"], "http://192.168.12.34:18081")
        self.assertEqual(values["APP_PORT"], "18081")

    def test_https_mode_and_legacy_domain_flag(self):
        for args in (("--mode", "https", "--domain", "blog.example.com"), ("--domain", "blog.example.com")):
            with self.subTest(args=args):
                (self.root / ".env").unlink(missing_ok=True)
                self.configure(*args)
                values = self.values()
                self.assertEqual(values["PUBLIC_BASE_URL"], "https://blog.example.com")
                self.assertEqual(values["PUBLIC_HOST"], "blog.example.com")
                self.assertEqual(values["APP_BIND_ADDRESS"], "127.0.0.1")
                self.assertEqual(values["ENVIRONMENT"], "production")

    def test_github_secret_is_read_from_private_file_and_not_printed(self):
        secret = "test-oauth-secret-42' #$literal-with-symbols_+-/="
        secret_file = self.base / "github secret.txt"
        secret_file.write_text(secret + "\n", encoding="utf-8")
        secret_file.chmod(0o600)
        result = self.configure("--github-client-id", "Iv1.test123", "--github-client-secret-file", str(secret_file), "--bootstrap-github-id", "123456")
        values = self.values()
        self.assertEqual(values["GITHUB_CLIENT_SECRET"], secret)
        self.assertEqual(values["GITHUB_CLIENT_ID"], "Iv1.test123")
        self.assertEqual(values["BOOTSTRAP_GITHUB_ID"], "123456")
        self.assertNotIn(secret, result.stdout + result.stderr)

    def test_github_secret_file_cannot_inject_another_env_variable(self):
        secret_file = self.base / "invalid github secret.txt"
        secret_file.write_text("secret\nAPP_BIND_ADDRESS=0.0.0.0\n", encoding="utf-8")
        secret_file.chmod(0o600)
        self.configure("--github-client-secret-file", str(secret_file), expected="failure")
        self.assertFalse((self.root / ".env").exists())

    def test_reconfigure_preserves_existing_env_and_never_prints_secrets(self):
        self.configure("--mode", "local")
        original = (self.root / ".env").read_bytes()
        values = self.values()
        result = self.run_script("configure.sh", ["--mode", "lan", "--lan-ip", "192.168.1.1", "--non-interactive"], expected=None)
        self.assertEqual((self.root / ".env").read_bytes(), original)
        self.assertEqual(stat.S_IMODE((self.root / ".env").stat().st_mode), 0o600)
        for key in SECRET_KEYS:
            self.assertNotIn(values[key], result.stdout + result.stderr)

    def test_install_without_daemon_does_not_create_env_or_start(self):
        self.painter("install", settings={"DAEMON": "0"}, expected="failure")
        self.assertFalse((self.root / ".env").exists())
        self.assert_no_start()

    def test_install_requires_compose_before_creating_env(self):
        self.painter("install", settings={"COMPOSE": "0"}, expected="failure")
        self.assertFalse((self.root / ".env").exists())
        self.assert_no_start()

    def test_install_refuses_existing_project_containers_without_env(self):
        self.painter("install", settings={"EXISTING_PROJECT": "1"}, expected="failure")
        self.assertFalse((self.root / ".env").exists())
        self.assert_no_start()
        checks = [command for command in self.commands() if command[0] == "ps"]
        self.assertTrue(checks, self.commands())
        self.assertTrue(any(any("label=com.docker.compose.project=" in arg for arg in command) for command in checks), checks)

    def test_install_refuses_existing_project_volumes_without_env(self):
        self.painter("install", settings={"EXISTING_VOLUME": "1"}, expected="failure")
        self.assertFalse((self.root / ".env").exists())
        self.assert_no_start()
        checks = [command for command in self.commands() if command[:2] == ["volume", "ls"]]
        self.assertTrue(checks, self.commands())
        self.assertTrue(any(any("label=com.docker.compose.project=" in arg for arg in command) for command in checks), checks)

    def test_install_configures_starts_and_checks_health_and_migrations(self):
        result = self.painter("install", "--timeout", "2")
        self.assert_private_env(result)
        self.assert_local_account_guidance(result, "http://localhost:8080")
        commands = self.commands()
        self.assertTrue(any(self.has_verb(command, "up") for command in commands), commands)
        inspections = [command for command in commands if command[0] == "inspect"]
        self.assertTrue(any(any("app" in arg for arg in command) for command in inspections), commands)
        self.assertTrue(any(any("migrate" in arg for arg in command) for command in inspections), commands)
        self.assertFalse(any(self.has_verb(command, "exec") and "backup" in command for command in commands), commands)
        self.assert_migrations_before_app_activation()

    def test_install_preserves_existing_env(self):
        self.configure()
        original = (self.root / ".env").read_bytes()
        result = self.painter("install", "--timeout", "2")
        self.assertEqual((self.root / ".env").read_bytes(), original)
        for value in self.values().values():
            if len(value) >= 32:
                self.assertNotIn(value, result.stdout + result.stderr)

    def test_install_rerun_exports_backup_before_stopping_existing_app(self):
        self.configure()
        original = (self.root / ".env").read_bytes()
        self.painter("install", "--timeout", "2")
        commands = self.commands()
        backup = next(index for index, command in enumerate(commands) if self.has_verb(command, "exec") and "backup" in command)
        export = next(index for index, command in enumerate(commands) if "cp" in command)
        build = next(index for index, command in enumerate(commands) if self.has_verb(command, "build"))
        self.assertLess(backup, export)
        self.assertLess(export, build)
        self.assertEqual((self.root / ".env").read_bytes(), original)
        self.assert_migrations_before_app_activation()

    def test_failed_install_rerun_backup_keeps_old_app_running(self):
        self.configure()
        self.painter("install", "--timeout", "1", settings={"BACKUP_FAIL": "1"}, expected="failure")
        self.assertTrue(any(self.has_verb(command, "exec") and "backup" in command for command in self.commands()), self.commands())
        self.assert_no_start()
        self.assert_no_stop()

    def test_install_rejects_symlinked_env_without_modifying_its_target(self):
        target = self.base / "outside-project.env"
        original = b"Do not replace or chmod this file.\n"
        target.write_bytes(original)
        target.chmod(0o640)
        (self.root / ".env").symlink_to(target)
        self.painter("install", expected="failure")
        self.assertEqual(target.read_bytes(), original)
        self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o640)
        self.assertTrue((self.root / ".env").is_symlink())
        self.assert_no_start()

    def test_install_with_image_pulls_and_never_builds_source(self):
        self.painter("install", "--image", IMAGE, "--timeout", "2")
        self.assertEqual(self.values()["PAINTER_IMAGE"], IMAGE)
        commands = self.commands()
        self.assertFalse(any(self.has_verb(command, "build") for command in commands), commands)
        pull = next(index for index, command in enumerate(commands) if self.has_verb(command, "pull"))
        up = next(index for index, command in enumerate(commands) if self.has_verb(command, "up"))
        self.assertLess(pull, up)
        self.assertIn("--no-build", commands[up])

    def test_explicit_build_installs_from_source(self):
        self.painter("install", "--build", "--timeout", "2")
        commands = self.commands()
        self.assertTrue(any(self.has_verb(command, "build") for command in commands), commands)
        self.assertFalse(any(self.has_verb(command, "pull") for command in commands), commands)

    def test_deployment_bundle_requires_image_instead_of_attempting_source_build(self):
        (self.root / "web/frontend/package-lock.json").unlink()
        result = self.painter("install", "--build", "--timeout", "1", expected="failure")
        self.assertIn("Source build requires the full checkout", result.stdout + result.stderr)
        self.assert_no_start()
        self.assert_no_stop()

    def test_https_install_distinguishes_app_health_from_public_https_readiness(self):
        result = self.painter("install", "--domain", "blog.example.com", "--timeout", "2")
        self.assert_local_account_guidance(result, "https://blog.example.com")
        output = result.stdout + result.stderr
        self.assertIn("Application healthy; HTTPS entry configured: https://blog.example.com", output)
        self.assertIn("DNS", output)
        self.assertIn("ACME", output)
        self.assertNotIn("Ready: https://blog.example.com", output)
        self.assertTrue(any(self.has_verb(command, "up") and "app" in command and "caddy" in command for command in self.commands()), self.commands())
        self.assert_migrations_before_app_activation()

    def test_failed_backup_blocks_update_build_and_start(self):
        self.configure()
        result = self.painter("update", "--timeout", "1", settings={"BACKUP_FAIL": "1"}, expected="failure")
        self.assert_no_start()
        self.assertTrue(any(self.has_verb(command, "exec") and "backup" in command for command in self.commands()), self.commands())
        self.assertFalse(any("cp" in command for command in self.commands()), self.commands())
        self.assertIn("backup", (result.stdout + result.stderr).lower())
        self.assert_no_stop()

    def test_failed_backup_export_blocks_update_build_and_start(self):
        self.configure()
        self.painter("update", "--timeout", "1", settings={"CP_FAIL": "1"}, expected="failure")
        self.assert_no_start()
        self.assertTrue(any("cp" in command for command in self.commands()), self.commands())
        self.assert_no_stop()

    def test_failed_source_build_keeps_old_app_running(self):
        self.configure()
        self.painter("update", "--timeout", "1", settings={"BUILD_FAIL": "1"}, expected="failure")
        self.assertTrue(any(self.has_verb(command, "build") for command in self.commands()), self.commands())
        self.assertFalse(any(self.has_verb(command, "up") for command in self.commands()), self.commands())
        self.assert_no_stop()

    def test_failed_stop_prevents_migration_and_application_activation(self):
        self.configure()
        result = self.painter("update", "--timeout", "1", settings={"STOP_FAIL": "1"}, expected="failure")
        self.assertTrue(any(self.has_verb(command, "stop") and "app" in command for command in self.commands()), self.commands())
        self.assertFalse(any(self.has_verb(command, "up") for command in self.commands()), self.commands())
        self.assertIn("migration was not started", result.stdout + result.stderr)

    def test_successful_update_exports_backup_before_build_and_start(self):
        self.configure()
        original = (self.root / ".env").read_bytes()
        self.painter("update", "--timeout", "2")
        commands = self.commands()
        backup = next(index for index, command in enumerate(commands) if self.has_verb(command, "exec") and "backup" in command)
        export = next(index for index, command in enumerate(commands) if "cp" in command)
        build = next(index for index, command in enumerate(commands) if self.has_verb(command, "build"))
        up = next(index for index, command in enumerate(commands) if self.has_verb(command, "up"))
        self.assertLess(backup, export)
        self.assertLess(export, build)
        self.assertLess(build, up)
        invocation = commands[backup]
        self.assertIn("-T", invocation)
        self.assertEqual(invocation[invocation.index("backup") - 1], "painter")
        directory = invocation[invocation.index("backup") + 1]
        self.assertTrue(directory.startswith("/var/backups/painter/"), directory)
        self.assertNotEqual(directory.rstrip("/"), "/var/backups/painter")
        self.assertEqual((self.root / ".env").read_bytes(), original)
        self.assertTrue(list((self.root / "backups").rglob("manifest.json")))
        self.assert_migrations_before_app_activation()

    def test_image_update_backs_up_then_pulls_before_start_and_commits_image(self):
        self.configure("--image", IMAGE)
        result = self.painter("update", "--image", NEW_IMAGE, "--timeout", "2")
        self.assert_local_account_guidance(result, "http://localhost:8080")
        commands = self.commands()
        backup = next(index for index, command in enumerate(commands) if self.has_verb(command, "exec") and "backup" in command)
        export = next(index for index, command in enumerate(commands) if "cp" in command)
        pull = next(index for index, command in enumerate(commands) if self.has_verb(command, "pull"))
        up = next(index for index, command in enumerate(commands) if self.has_verb(command, "up"))
        self.assertLess(backup, export)
        self.assertLess(export, pull)
        self.assertLess(pull, up)
        self.assertIn("--no-build", commands[up])
        self.assertFalse(any(self.has_verb(command, "build") for command in commands), commands)
        self.assertEqual(self.values()["PAINTER_IMAGE"], NEW_IMAGE)
        self.assertEqual(stat.S_IMODE((self.root / ".env").stat().st_mode), 0o600)
        observations = self.observations()
        health_checks = [index for index, item in enumerate(observations) if item["args"][0] == "inspect" and any("Health" in arg for arg in item["args"])]
        self.assertTrue(health_checks, commands)
        for item in observations[:health_checks[-1] + 1]:
            self.assertEqual(item["configured_image"], IMAGE, item)
        self.assert_migrations_before_app_activation()

    def test_failed_image_update_keeps_previous_image_and_env_unchanged(self):
        self.configure("--image", IMAGE)
        original = (self.root / ".env").read_bytes()
        self.painter("update", "--image", NEW_IMAGE, "--timeout", "1", settings={"HEALTH": "unhealthy"}, expected="failure")
        self.assertEqual((self.root / ".env").read_bytes(), original)
        self.assertEqual(self.values()["PAINTER_IMAGE"], IMAGE)
        self.assertTrue(any(self.has_verb(command, "up") for command in self.commands()), self.commands())
        self.assertTrue(any(command[0] == "inspect" and any("Health" in arg for arg in command) for command in self.commands()), self.commands())

    def test_failed_image_pull_keeps_previous_image_and_does_not_start(self):
        self.configure("--image", IMAGE)
        original = (self.root / ".env").read_bytes()
        self.painter("update", "--image", NEW_IMAGE, "--timeout", "1", settings={"PULL_FAIL": "1"}, expected="failure")
        self.assertEqual((self.root / ".env").read_bytes(), original)
        self.assert_no_start()
        self.assertTrue(any(self.has_verb(command, "pull") for command in self.commands()), self.commands())
        self.assert_no_stop()

    def test_source_update_overrides_a_previously_configured_release_image(self):
        self.configure("--image", IMAGE)
        self.painter("update", "--build", "--timeout", "2")
        commands = self.commands()
        self.assertFalse(any(self.has_verb(command, "pull") for command in commands), commands)
        self.assertTrue(any(self.has_verb(command, "build") for command in commands), commands)
        for item in self.observations():
            if self.has_verb(item["args"], "build") or self.has_verb(item["args"], "up"):
                self.assertIn(item["process_image"], ("", "painter-next:local"), "Source mode must override the release image from .env: " + repr(item))
        self.assertEqual(self.values()["PAINTER_IMAGE"], "")
        self.assert_migrations_before_app_activation()

    def test_backups_use_a_new_directory_for_each_invocation(self):
        self.configure()
        self.painter("backup")
        self.painter("backup")
        directories = [command[command.index("backup") + 1] for command in self.commands() if self.has_verb(command, "exec") and "backup" in command]
        self.assertEqual(len(directories), 2)
        self.assertEqual(len(set(directories)), 2)

    def test_logs_and_stop_never_delete_volumes(self):
        self.configure()
        self.painter("logs")
        self.painter("stop")
        commands = self.commands()
        self.assertTrue(any(self.has_verb(command, "logs") for command in commands), commands)
        self.assertTrue(any(self.has_verb(command, "stop") for command in commands), commands)
        for command in commands:
            self.assertNotIn("down", command)
            self.assertNotIn("-v", command)
            self.assertNotIn("--volumes", command)
            self.assertNotIn("rm", command)

    def test_status_uses_compose_without_starting_services(self):
        self.configure()
        result = self.painter("status")
        self.assertTrue(any(self.has_verb(command, "ps") for command in self.commands()), self.commands())
        self.assert_no_start()
        for key in SECRET_KEYS:
            self.assertNotIn(self.values()[key], result.stdout + result.stderr)

    def test_setup_token_is_explicit_and_does_not_start_or_mutate_services(self):
        self.configure()
        before = (self.root / ".env").read_bytes()
        result = self.painter("setup-token")
        self.assertIn("mock-operator-installation-code", result.stdout)
        self.assertEqual((self.root / ".env").read_bytes(), before)
        self.assertTrue(any(command[-5:] == ["exec", "-T", "app", "painter", "setup-token"] for command in self.commands()))
        self.assert_no_start()

    def test_setup_token_failure_does_not_claim_success_or_start_services(self):
        self.configure()
        result = self.painter("setup-token", settings={"SETUP_TOKEN_FAIL": "1"}, expected="failure")
        self.assertNotIn("mock-operator-installation-code", result.stdout)
        self.assert_no_start()

    def test_existing_env_is_never_evaluated_as_shell_code(self):
        self.configure()
        dangerous = self.base / "env-executed-marker"
        with (self.root / ".env").open("a", encoding="utf-8") as handle:
            handle.write("UNUSED_VALUE=$(touch '" + str(dangerous) + "')\n")
        self.painter("status")
        self.assertFalse(dangerous.exists())

    def test_unhealthy_app_exits_nonzero_without_success_claim(self):
        self.configure()
        result = self.painter("install", "--timeout", "1", settings={"HEALTH": "unhealthy"}, expected="failure")
        output = (result.stdout + result.stderr).lower()
        self.assertTrue("health" in output or "unhealthy" in output, output)
        self.assertNotIn("installation complete", output)
        self.assertNotIn("installed successfully", output)

    def test_failed_migration_exits_nonzero_without_success_claim(self):
        self.configure()
        result = self.painter("install", "--timeout", "1", settings={"MIGRATE_EXIT": "1"}, expected="failure")
        output = (result.stdout + result.stderr).lower()
        self.assertIn("migrat", output)
        self.assertNotIn("installation complete", output)
        self.assertNotIn("installed successfully", output)
        self.assertTrue(any(self.has_verb(command, "up") and "migrate" in command for command in self.commands()), self.commands())
        self.assertFalse(any(self.has_verb(command, "up") and "app" in command for command in self.commands()), self.commands())
        stops = [index for index, command in enumerate(self.commands()) if self.has_verb(command, "stop") and "app" in command]
        migration = next(index for index, command in enumerate(self.commands()) if self.has_verb(command, "up") and "migrate" in command)
        self.assertEqual(len(stops), 1)
        self.assertLess(stops[0], migration)
        self.assertIn("old application is stopped", output)

    def test_health_timeout_exits_nonzero_without_success_claim(self):
        self.configure()
        result = self.painter("install", "--timeout", "1", settings={"HEALTH": "starting"}, expected="failure")
        output = (result.stdout + result.stderr).lower()
        self.assertTrue("health" in output or "tim" in output, output)
        self.assertNotIn("installation complete", output)
        self.assertNotIn("installed successfully", output)

    def test_unsafe_configuration_input_is_rejected_without_creating_env(self):
        dangerous = self.base / "injected-marker"
        cases = (
            ("--mode", "unknown"),
            ("--port", "0"), ("--port", "65536"), ("--port", "nope"),
            ("--port", "8080:8080"),
            ("--mode", "lan"),
            ("--mode", "lan", "--lan-ip", "0.0.0.0"),
            ("--mode", "lan", "--lan-ip", "::"),
            ("--mode", "lan", "--lan-ip", "192.168.0.999"),
            ("--domain", "https://example.com"),
            ("--domain", "example.com/path"),
            ("--domain", "example.com;touch " + str(dangerous)),
            ("--domain", "$(touch " + str(dangerous) + ")"),
            ("--github-client-id", "valid\nAPP_BIND_ADDRESS=0.0.0.0"),
            ("--bootstrap-github-id", "123;456"),
            ("--github-client-secret-file", str(self.base / "missing-secret")),
            ("--unknown-option",),
        )
        for args in cases:
            with self.subTest(args=args):
                (self.root / ".env").unlink(missing_ok=True)
                self.configure(*args, expected="failure")
                self.assertFalse((self.root / ".env").exists())
                self.assertFalse(dangerous.exists())
        self.assertEqual(self.commands(), [])

    def test_invalid_command_or_timeout_does_not_contact_docker(self):
        for command, args in (("destroy", ()), ("install", ("--timeout", "-1")), ("update", ("--timeout", "invalid")), ("stop", ("--unknown-option",))):
            with self.subTest(command=command, args=args):
                self.painter(command, *args, expected="failure")
        self.assertEqual(self.commands(), [])

    def test_image_and_build_flags_are_mutually_exclusive(self):
        for command in ("install", "update"):
            with self.subTest(command=command):
                self.painter(command, "--image", IMAGE, "--build", expected="failure")
        self.assertEqual(self.commands(), [])

    def test_help_through_both_entrypoints_never_contacts_docker(self):
        cli_help = self.painter("install", "--help")
        installer_help = self.run_script("../install.sh", ["--help"])
        self.assertIn("Painter deployment and operations", cli_help.stdout)
        self.assertIn("Painter deployment and operations", installer_help.stdout)
        self.assertFalse((self.root / ".env").exists())
        self.assertEqual(self.commands(), [])

    def test_deployment_package_has_exact_public_whitelist_and_image_hint(self):
        secret = b"private-fixture-data-must-never-be-packaged\n"
        for name in (".env", "backups/private.dump", "media/private.jpg", "web/assets/private.js", "internal/private.go", "deploy/private-secret.txt"):
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(secret)
        output = self.base / "release bundle.tar.gz"
        result = self.package(output)
        with tarfile.open(output) as archive:
            files = {member.name.removeprefix("painter-next/") for member in archive.getmembers() if member.isfile()}
            self.assertEqual(files, BUNDLE_FILES)
            self.assertTrue(all(member.isdir() or member.isfile() for member in archive.getmembers()))
            for member in archive.getmembers():
                self.assertTrue(member.name == "painter-next" or member.name.startswith("painter-next/"), member.name)
                if member.isfile():
                    self.assertNotIn(secret, archive.extractfile(member).read(), member.name)
            hint = archive.extractfile("painter-next/deploy/release-image.txt").read().decode()
            self.assertEqual(hint, IMAGE + "\n")
            for name in ("local-accounts.md", "ACCOUNT_SECURITY.md", "IDENTITY_PROVIDER.md", "AUTOMATIC_BACKUP_RECOVERY.md"):
                self.assertEqual(archive.extractfile("painter-next/docs/" + name).read(), (PROJECT / "docs" / name).read_bytes())
        checksum = output.with_name(output.name + ".sha256").read_text()
        self.assertEqual(checksum, hashlib.sha256(output.read_bytes()).hexdigest() + "  " + output.name + "\n")
        self.assertNotIn(secret.decode().strip(), result.stdout + result.stderr)
        self.assertEqual(self.commands(), [])

    def test_deployment_package_never_overwrites_existing_output_or_checksum(self):
        original = b"preserve this existing artifact\n"
        for existing in ("output", "checksum", "symlink"):
            with self.subTest(existing=existing):
                output = self.base / (existing + ".tar.gz")
                checksum = output.with_name(output.name + ".sha256")
                if existing == "output":
                    output.write_bytes(original)
                elif existing == "checksum":
                    checksum.write_bytes(original)
                else:
                    target = self.base / "symlink-target"
                    target.write_bytes(original)
                    output.symlink_to(target)
                self.package(output, expected="failure")
                if existing == "checksum":
                    self.assertFalse(output.exists())
                    self.assertEqual(checksum.read_bytes(), original)
                else:
                    self.assertEqual(output.read_bytes(), original)
                    self.assertFalse(checksum.exists())
                    if existing == "symlink":
                        self.assertTrue(output.is_symlink())
        self.assertEqual(self.commands(), [])

    def test_extracted_bundle_install_uses_release_image_hint_without_build(self):
        output = self.base / "installable bundle.tar.gz"
        self.package(output)
        extracted = self.base / "extracted bundle"
        with tarfile.open(output) as archive:
            for member in archive.getmembers():
                if member.isdir():
                    continue
                self.assertTrue(member.isfile(), member.name)
                self.assertTrue(member.name.startswith("painter-next/"), member.name)
                self.assertNotIn("..", Path(member.name).parts)
                self.assertIn(member.name.removeprefix("painter-next/"), BUNDLE_FILES)
                target = extracted / member.name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(archive.extractfile(member).read())
                target.chmod(member.mode & 0o777)
        self.root = extracted / "painter-next"
        self.env["MOCK_PROJECT_ROOT"] = str(self.root)
        self.assertFalse((self.root / "Dockerfile").exists())
        self.assertFalse((self.root / "go.mod").exists())
        self.assertFalse((self.root / ".env").exists())
        result = self.run_script("../install.sh", ["--non-interactive", "--timeout", "2"])
        self.assert_local_account_guidance(result, "http://localhost:8080")
        self.assertTrue((self.root / "docs/local-accounts.md").is_file())
        self.assertEqual(self.values()["PAINTER_IMAGE"], IMAGE)
        self.assertTrue(any(self.has_verb(command, "pull") for command in self.commands()), self.commands())
        self.assertFalse(any(self.has_verb(command, "build") for command in self.commands()), self.commands())
        self.assert_migrations_before_app_activation()

    def test_package_rejects_unsafe_or_unpinned_image_without_writing_archive(self):
        for index, image in enumerate(("example/painter:latest", "https://registry.example/painter:v1", "example/painter:v1;touch marker")):
            with self.subTest(image=image):
                output = self.base / ("invalid-image-" + str(index) + ".tar.gz")
                self.package(output, image=image, expected="failure")
                self.assertFalse(output.exists())
                self.assertFalse(output.with_name(output.name + ".sha256").exists())
        self.assertEqual(self.commands(), [])

    def test_doctor_checks_running_health_and_fresh_readiness_without_service_changes(self):
        self.configure()
        result = self.painter("doctor")
        self.assertIn("Deployment checks passed", result.stdout)
        self.assertTrue(any(command[-5:] == ["exec", "-T", "app", "painter", "healthcheck"] for command in self.commands()))
        self.assert_no_start()
        self.assert_no_stop()
        for key in SECRET_KEYS:
            self.assertNotIn(self.values()[key], result.stdout + result.stderr)

    def test_doctor_does_not_report_stopped_or_unhealthy_deployment_as_ready(self):
        self.configure()
        for settings in ({"HEALTH": "unhealthy"}, {"DB_STATUS": "exited"}, {"APP_STATUS": "exited"}):
            with self.subTest(settings=settings):
                result = self.painter("doctor", settings=settings, expected="failure")
                self.assertNotIn("Deployment checks passed", result.stdout)
        self.assert_no_start()
        self.assert_no_stop()

    def test_doctor_preserves_fresh_readiness_failure_exit_code(self):
        self.configure()
        self.painter("doctor", settings={"READINESS_FAIL": "1", "FAIL_CODE": "47"}, expected=47)
        self.assert_no_start()

    def test_start_only_starts_existing_containers_database_first_without_migration(self):
        self.configure()
        self.painter("start", "--timeout", "2")
        starts = [command for command in self.commands() if self.has_verb(command, "start")]
        self.assertEqual([command[command.index("start") + 1:] for command in starts], [["db"], ["app"]])
        self.assert_no_start()  # no build/up or implicit schema migration
        self.assertFalse(any("migrate" in command for command in starts))

    def test_unhealthy_database_prevents_application_start(self):
        self.configure()
        self.painter("start", "--timeout", "1", settings={"DB_HEALTH": "unhealthy"}, expected="failure")
        starts = [command for command in self.commands() if self.has_verb(command, "start")]
        self.assertEqual(len(starts), 1)
        self.assertEqual(starts[0][-2:], ["start", "db"])

    def test_restart_only_restarts_app_and_preserves_failure_code(self):
        self.configure()
        self.painter("restart", "--timeout", "2")
        commands = [command for command in self.commands() if self.has_verb(command, "restart")]
        self.assertEqual(len(commands), 1)
        self.assertEqual(commands[0][-4:], ["restart", "--timeout", "30", "app"])
        self.painter("restart", settings={"RESTART_FAIL": "1", "FAIL_CODE": "42"}, expected=42)
        self.assert_no_start()

    def test_backup_captures_private_configuration_and_never_prints_its_secrets(self):
        self.configure()
        original = (self.root / ".env").read_bytes()
        result = self.painter("backup")
        backups = list((self.root / "backups").iterdir())
        self.assertEqual(len(backups), 1)
        backup = backups[0]
        self.assertEqual((backup / "deployment.env").read_bytes(), original)
        self.assertEqual(stat.S_IMODE((backup / "deployment.env").stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(backup.stat().st_mode), 0o700)
        self.assertEqual((backup / "compose.yaml").read_bytes(), (self.root / "compose.yaml").read_bytes())
        for key in SECRET_KEYS:
            self.assertNotIn(self.values()[key], result.stdout + result.stderr)

    def test_backups_inventory_works_without_docker_and_ignores_unsafe_entries(self):
        self.configure()
        exported = self.root / "backups" / "20261102T010203Z-abc12345"
        exported.mkdir()
        (exported / "manifest.json").write_text("{}")
        outside = self.base / "outside"
        outside.mkdir()
        (outside / "manifest.json").write_text("{}")
        (self.root / "backups" / "unsafe-link").symlink_to(outside, target_is_directory=True)
        result = self.painter("backups", settings={"DAEMON": "0"})
        self.assertIn(exported.name, result.stdout)
        self.assertNotIn("unsafe-link", result.stdout)
        self.assertEqual(self.commands(), [])

    def test_verify_backup_is_isolated_and_preserves_failure_exit_status(self):
        self.configure()
        name = "20261102T010203Z-abc12345"
        self.painter("verify-backup", "--backup", name)
        runs = [command for command in self.commands() if self.has_verb(command, "run")]
        self.assertEqual(len(runs), 1)
        self.assertEqual(runs[0][-7:], ["run", "--rm", "--no-deps", "-T", "ops", "restore-check", "/var/backups/painter/" + name])
        self.painter("verify-backup", "--backup", name, settings={"RESTORE_FAIL": "1", "FAIL_CODE": "43"}, expected=43)
        self.assert_no_start()
        self.assert_no_stop()

    def test_verify_backup_rejects_paths_or_injection_before_contacting_docker(self):
        for name in ("..", "../outside", "/tmp/foo", "name;touch marker", "bad\nname", "-option"):
            with self.subTest(name=name):
                self.painter("verify-backup", "--backup", name, expected="failure")
        self.painter("verify-backup", expected="failure")
        self.painter("status", "--backup", "unused", expected="failure")
        self.painter("restart", "--build", expected="failure")
        self.assertEqual(self.commands(), [])

    def test_backup_pull_and_compose_failures_preserve_original_subprocess_status(self):
        self.configure()
        for command, options, settings in (
            ("backup", [], {"BACKUP_FAIL": "1"}),
            ("update", ["--image", NEW_IMAGE], {"PULL_FAIL": "1"}),
            ("status", [], {"CONFIG_FAIL": "1"}),
            ("status", [], {"DAEMON": "0"}),
        ):
            with self.subTest(command=command, settings=settings):
                self.painter(command, *options, settings={**settings, "FAIL_CODE": "46"}, expected=46)

    def test_migration_failure_preserves_container_exit_code(self):
        self.configure()
        self.painter("install", "--timeout", "2", settings={"MIGRATE_EXIT": "53"}, expected=53)
        self.assertFalse(any(self.has_verb(command, "up") and "app" in command for command in self.commands()))

    def test_maintenance_entrypoint_forwards_help_and_noninteractive_requires_command(self):
        result = self.run_script("../maintain.sh", ["--help"], with_root=False)
        self.assertIn("Painter deployment and operations", result.stdout)
        result = self.run_script("../maintain.sh", [], expected="failure", with_root=False)
        self.assertIn("maintenance menu", result.stderr)
        self.assertEqual(self.commands(), [])

    def test_failed_existing_app_inspection_blocks_install_rerun_before_any_activation(self):
        self.configure()
        self.painter("install", "--timeout", "2", settings={"PS_FAIL":"1", "FAIL_CODE":"44"}, expected=44)
        self.assert_no_start()
        self.assert_no_stop()


if __name__ == "__main__":
    unittest.main()
