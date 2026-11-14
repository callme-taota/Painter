#!/usr/bin/env bash
# CI/operator helper. It does not publish images, upload bundles, or read .env.
set -euo pipefail
umask 077
DEFAULT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
source "$DEFAULT_ROOT/scripts/lib/deploy.sh"
output=''; image=''
while (($#)); do case "$1" in --output) [[ $# -ge 2 ]] || die '--output needs a new archive path'; output=$2; shift 2;; --image) [[ $# -ge 2 ]] || die '--image needs an existing published tag or digest'; image=$2; shift 2;; --help|-h) printf '%s\n' 'Usage: scripts/package-deploy.sh --image EXISTING_REGISTRY_IMAGE:VERSION --output /path/to/new-bundle.tar.gz' 'Build/publish that image in your CI first. This only packages a credential-free installer.'; exit 0;; *) die "Unknown option: $1";; esac; done
[[ -n "$output" && -n "$image" ]] || die '--image and --output are required'; valid_image "$image" || die 'Use a real explicit tag or sha256 digest'; need tar; need sha256sum
[[ ! -e "$output" && ! -L "$output" ]] || die 'Output already exists; it was not replaced.'
[[ ! -e "$output.sha256" && ! -L "$output.sha256" ]] || die 'Output checksum already exists; it was not replaced.'
stage=$(mktemp -d); trap 'rm -rf -- "$stage"' EXIT
bundle="$stage/painter-next"; mkdir -p "$bundle/scripts/lib" "$bundle/deploy" "$bundle/docs"
for file in install.sh maintain.sh compose.yaml; do cp -- "$DEFAULT_ROOT/$file" "$bundle/$file"; done
for file in painter configure.sh install-docker.sh package-deploy.sh backup-schedule.py recover-backup.py; do cp -- "$DEFAULT_ROOT/scripts/$file" "$bundle/scripts/$file"; done
cp -- "$DEFAULT_ROOT/scripts/lib/deploy.sh" "$bundle/scripts/lib/deploy.sh"
for file in Caddyfile init-roles.sh example.env images.lock.json; do cp -- "$DEFAULT_ROOT/deploy/$file" "$bundle/deploy/$file"; done
for file in DEPLOYMENT.md deployment-tool.md admin-cli.md local-accounts.md INSTALLATION_V7.md GITHUB_LOGIN_V7.md ACCOUNT_SECURITY.md IDENTITY_PROVIDER.md AUTOMATIC_BACKUP_RECOVERY.md; do cp -- "$DEFAULT_ROOT/docs/$file" "$bundle/docs/$file"; done
printf '%s\n' "$image" > "$bundle/deploy/release-image.txt"
printf '%s\n' 'Painter deployment bundle: application image is specified in deploy/release-image.txt.' 'No source compiler, application credentials, database, media, or published download URL is included.' 'Run ./install.sh and follow docs/DEPLOYMENT.md. The image must already exist in your registry.' > "$bundle/README.txt"
mkdir -p -- "$(dirname "$output")"
# noclobber reserves a new output; a failed tar leaves an incomplete archive to inspect.
(set -C; : > "$output") || die 'Output appeared while packaging; it was preserved.'
tar -C "$stage" -czf "$output" painter-next
(cd "$(dirname "$output")" && sha256sum "$(basename "$output")") > "$output.sha256"
printf 'Created deployment bundle and checksum: %s\n' "$output"
