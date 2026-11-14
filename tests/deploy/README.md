# Deployment contract tests

Run from the repository root:

```sh
python3 -m unittest discover -s tests/deploy -v
```

The suite uses Python's standard library, Bash, OpenSSL, and normal local archive
tools (`tar` and `sha256sum`). Docker does not need to be installed or running.
Each test copies deployment scripts, deployment resources, entrypoints, and
selected build-eligibility files into an independent temporary project whose
path contains spaces. Package tests also copy the deployment and local-account
guides, including first-run installation, GitHub configuration and account security.
The suite never copies or reads the real project's `.env`.

A strict `docker` double is placed first on `PATH`. It logs argument arrays and
simulates daemon/Compose failures, backup/export failures, migration exit codes,
and application health. Backup exports are confined to the temporary project.
Unexpected Docker verbs fail. Other tools that could contact services or mutate
a checkout (`git`, `curl`, `wget`, `sudo`, and `systemctl`) are guarded so a
deployment test cannot silently call them.

The contracts cover local/LAN/HTTPS configuration and compatibility flags,
private independent secrets, preserving existing configuration, safe argument
validation, Docker preflight, backup-before-update ordering, backup uniqueness,
failure propagation, health and migration verification, and read-only status /
volume-preserving stop behavior. Image deployments must pull without building;
an image update must preserve the previous configured image until all health
checks succeed and leave it unchanged after a failed pull or health check. An
existing application is stopped only after build/pull succeeds and before the
new migration runs. Rerunning installation on an existing application requires
a successful backup and export first.

Package tests verify the exact public-file whitelist, image hint, archive
checksum, protection of existing output/checksum files, and installation from
an extracted bundle without source compilation. HTTPS output must distinguish
application health from public DNS and ACME verification. Help commands must not
contact Docker.

Successful installation and update output points to `/setup`, the explicit
server-only `setup-token` operation, the offline first-owner CLI fallback, no
default password, and optional GitHub OAuth.
The installer itself never creates a local account. Bundle tests also check that
the local-account operations guide is included unchanged in the exact whitelist.

These tests verify the deployment script's decisions and command ordering. A
separate real-Docker smoke test is needed to verify image building, Compose
interpolation, container networking, and actual backup/restore contents.

Maintenance coverage includes the `maintain.sh` terminal entrypoint, `doctor`, volume-preserving
`start`/application-only `restart`, offline `backups` inventory and isolated
`verify-backup`. Contracts cover fresh HTTP readiness, owner-only configuration
snapshots inside private backups, path traversal rejection, subprocess/container
exit-code propagation and the expanded public bundle whitelist. The maintenance
and administrator CLI guides ship in the public bundle; credentials never do.
