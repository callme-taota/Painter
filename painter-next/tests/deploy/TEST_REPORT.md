# Deployment test report

Date: 2026-10-25

## Mock suite

```text
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests/deploy -v
Ran 45 tests in 15.090s
OK
```

All deployment invocations ran against temporary project copies and the strict
Docker double. No real Docker daemon, container, service, external Git operation,
or network operation was invoked. No dependencies were installed. The real
project `.env` was neither copied nor read. Temporary fixture paths contain
spaces to verify argument quoting. Package creation used real local archive
tools and temporary fixture files only.

Verified contracts:

- Local, LAN, and HTTPS configuration; legacy flags; custom ports; rejection of
  invalid modes, unsafe origins, wildcard/IPv6 LAN bindings, invalid ports,
  missing secret files, multiline values, and unknown arguments.
- Independent generated secrets, application-compatible 32-byte base64 keys,
  `.env` mode `0600`, private backup directory, secret-file special characters,
  and preservation of existing configuration and symlink targets.
- Docker daemon and Compose preflight before configuration creation; rejection
  of existing project containers or volumes when `.env` is missing.
- Source build and image pull paths; mutually exclusive build/image options;
  source mode overriding a previously configured release image.
- Backup execution and host export before build/pull and activation; unique
  backup directories; existing-install reruns also require a backup/export;
  backup/export/build/pull failures leave the previous application running.
- Successful build/pull before `stop --timeout 30 app`, then forced migration-
  container recreation, successful migration exit verification, separate
  application activation, and health verification. A failed stop prevents
  migration. A failed migration leaves the old application stopped and does not
  activate it against the changed schema.
- A new image is written only after health succeeds; failed pull or health
  preserves the previous image and complete `.env` contents.
- Nonzero failure on migration errors, unhealthy applications, and health
  timeouts; status/log operations do not start services; stop retains volumes.
- HTTPS success output describes application health and entry configuration,
  with DNS and ACME still to verify. CLI and installer help contact no Docker.
- Missing source-build files reject local compilation in a deployment bundle.
- Deployment packages contain exactly the public-file whitelist, image hint,
  and SHA-256 sidecar; exclude synthetic `.env`, backups, media, assets, Go source,
  and extra deployment secrets; preserve existing output/checksum files and
  symlink targets; and reject invalid or unpinned image references.
- A package extracted into a fresh directory runs its installer using the image
  hint and the mocked pull path, without a source build or explicit image flag.

The suite verifies script behavior, package contents, and command ordering. It
does not run containers, verify real Compose interpolation or release-image
availability, install Docker/dependencies, or validate container networking,
actual backup archive contents, or restore behavior. Those require separate
integration checks. No image or package was published by this test run.

## Real Compose read-only checks

Separate from the 45 mocked tests, the installed Docker CLI 29.8.2 and Compose
5.6.0 successfully parsed temporary configurations using `docker compose config
--format json` and `docker compose config --environment`. These checks used
synthetic configuration and secrets; the real project `.env` was not read.

- Local mode resolved the application binding to `127.0.0.1` and the public URL
  to `http://localhost:<port>`.
- LAN mode resolved the application binding specifically to `192.168.2.209`
  and the public URL to the same IPv4 address, without a wildcard or IPv6 bind.
- HTTPS mode kept the application on `127.0.0.1`, enabled the Caddy profile, and
  normalized an uppercase input domain into the lowercase HTTPS public URL.
- A custom Compose project resolved its default source image to
  `painter-demo:local`, rather than sharing the default project's image tag.
- A synthetic GitHub secret containing dollar signs, hash signs, apostrophes,
  backslashes, and spaces survived dotenv interpolation exactly.
  `config --environment` matched the original value; the canonical config
  printer's doubled dollar signs were accounted for as its re-readable output
  escaping. No secret values are recorded in this report.

`bash -n` also passed for the installer, deployment scripts, shared library, and
database-role initialization script. A read-only `docker info` check returned
exit status 1 because no Docker daemon was available. No containers were
started, no Docker installation was attempted, and no current native
application, database, media, or frontend assets were changed by these checks.
Actual container startup, public HTTPS/DNS/ACME, image availability, and NAS
operation remain unverified in this environment.

## Interactive local-server default follow-up

Date: 2026-10-25

The earlier 45-test and read-only Compose results above are retained as the
previous verification record. This follow-up changes only the first interactive
access-mode prompt's default from `local` to `lan`.

```text
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests/deploy -v
Ran 48 tests in 13.906s
OK

bash -n scripts/lib/deploy.sh scripts/configure.sh
exit status 0
```

Three new regression tests cover the following contracts:

- A real pseudo-terminal invocation displays `lan` as the default. After Enter,
  it displays a deterministic detected private IPv4 (`192.168.12.34`) and waits
  for the address response before proceeding. The resulting binding is that one
  IPv4, with a matching public URL and private, independent generated secrets.
- Typing `local` at the interactive mode prompt still selects loopback. Explicit
  `--local`, `--mode local`, `--mode lan` with an IPv4, `--domain`, and `--mode
  https` with a domain retain their selections and skip the access-mode prompt.
- A non-TTY invocation with no mode or non-interactive flag still defaults to
  `local`. The original explicit non-interactive default test also remains green.

The pseudo-terminal driver uses only Python's standard library and answers each
prompt after observing it. LAN detection is supplied by a temporary `ip` double;
no host address detection or network request is needed for these tests. All 48
tests use temporary project copies and the strict Docker double. The real project
`.env` was neither copied nor read, and no dependency installation, real Docker
operation, current service, firewall, or deployment configuration was changed.
No real Compose or container-startup check was repeated in this follow-up.

## Local-account deployment guidance follow-up

Date: 2026-10-28

The earlier 45-test, 48-test, and read-only Compose records above are preserved.
This follow-up aligns deployment guidance and bundle contents with the existing
local-account CLI; it adds no automatic account creation or deployment option.

```text
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests/deploy -v
Ran 48 tests in 15.331s
OK

bash -n install.sh scripts/painter scripts/configure.sh scripts/package-deploy.sh scripts/lib/deploy.sh
exit status 0

All 10 Bash code blocks in docs/DEPLOYMENT.md passed bash -n.
```

Updated assertions verify that successful fresh installation, image update,
HTTPS installation, and installation from an extracted bundle point to local
`/login`, explicit offline first-owner creation, no default account or password,
production registration closed by default, and optional GitHub OAuth. The strict
Docker log confirms the installer does not invoke `create-local-user` itself.
Pseudo-terminal tests retain the LAN-default and explicit-mode compatibility
contracts while checking the new optional-GitHub prompt.

The exact bundle whitelist now includes `docs/local-accounts.md`. Its archived
contents match the source guide, and the extracted bundle retains it. Existing
checks still reject unpinned images and accidental data/secret inclusion, retain
output and checksum protections, and verify the mocked image-pull path without
claiming that a public image or downloadable release exists.

The first-owner examples were statically checked against the current CLI,
Dockerfile, Compose app environment, and database grants. They use hidden Bash
input or a pre-existing private host file, passing the password to
`docker compose exec -T app painter create-local-user ... --password-stdin`.
The documented shell input preserves spaces, disables tracing/automatic export,
and supplies EOF; host file redirection does not pretend that a host path is a
container `--password-file` path. The examples were syntax-checked, not executed
against a real account or database by this test run.

All 48 tests ran in independent temporary project copies with the strict Docker
double. The real project `.env` was neither copied nor read. No network operation,
dependency installation, real Docker command, current service, database, firewall,
or deployment configuration was changed. No real container or public-image
availability check was performed, and no image or installation package was
published by this test run.

## V15 maintenance tool verification — 2026-11-02

```text
python3 -m unittest discover -s tests/deploy -q
Ran 64 tests in 21.786s
OK

bash -n maintain.sh install.sh scripts/painter scripts/configure.sh scripts/package-deploy.sh scripts/lib/deploy.sh
exit status 0
```

The final suite includes the maintenance terminal entrypoint and its direct
command form, fresh readiness diagnosis, database-first existing-container start,
application-only restart, offline backup inventory, safe backup-name validation,
isolated restore-check invocation and original process/container failure codes.
Backups include the original deployment configuration (0600 inside a 0700
directory) and Compose snapshot without printing secrets. The exact public
package whitelist includes `maintain.sh`, `deployment-tool.md` and `admin-cli.md`.

These 64 cases used only copied temporary projects and the Docker double. No
real Docker deployment or restore was executed; the formal LAN Go service,
PostgreSQL database and private environment file were not accessed by this suite.
