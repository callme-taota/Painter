#!/usr/bin/env bash
# Optional explicit host provisioning, based on Docker's official apt instructions.
# This script does not remove conflicting packages, data, users, or volumes.
set -euo pipefail
umask 077
fail() { printf 'Docker setup: %s\n' "$*" >&2; exit 1; }
yes=0
case "${1:-}" in --yes) yes=1; shift;; --help|-h) printf '%s\n' 'Usage: scripts/install-docker.sh [--yes]' 'Opt-in official Docker apt repository installation, Ubuntu/Debian with systemd only.'; exit 0;; '') ;; *) fail 'Unknown option';; esac
(($#==0)) || fail 'Unexpected arguments'
[[ "$(uname -s)" == Linux && -r /etc/os-release ]] || fail 'Only Ubuntu/Debian Linux is supported; NAS must provide Docker/Compose separately.'
# /etc/os-release is the OS-owned configuration, not a user dotenv file.
. /etc/os-release
case "${ID:-}:${VERSION_ID:-}" in ubuntu:22.04|ubuntu:24.04|ubuntu:26.04|ubuntu:26.10) distro=ubuntu; suite=${UBUNTU_CODENAME:-${VERSION_CODENAME:-}};; debian:12|debian:13) distro=debian; suite=${VERSION_CODENAME:-};; *) fail 'Unsupported OS release. Follow the distribution-specific official Docker instructions.';; esac
[[ "$suite" =~ ^[a-z]+$ ]] || fail 'Cannot determine supported apt codename'
command -v systemctl >/dev/null || fail 'systemd is required for automatic setup; install Docker manually on this host.'
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1 && docker info >/dev/null 2>&1; then printf '%s\n' 'Docker and Compose are already usable; no packages or daemon settings changed.'; exit 0; fi
if ((yes==0)); then [[ -t 0 ]] || fail 'Use --yes for an explicitly authorized non-interactive installation.'; printf 'Install official Docker apt packages and enable/start its system service? [y/N] ' >&2; IFS= read -r reply; [[ "$reply" =~ ^[yY]$ ]] || fail 'Cancelled before changes.'; fi
root=()
if ((EUID!=0)); then command -v sudo >/dev/null || fail 'root or sudo privileges are required.'; if ((yes)); then sudo -n true 2>/dev/null || fail 'Non-interactive sudo is unavailable. Run this explicit command as root or install Docker manually.'; else sudo -v; fi; root=(sudo); fi
command -v apt-get >/dev/null && command -v dpkg-query >/dev/null || fail 'apt/dpkg are required.'
conflicts=()
for package in docker.io docker-compose docker-compose-v2 docker-doc docker-buildx podman-docker containerd runc; do state=$(dpkg-query -W -f='${db:Status-Status}' "$package" 2>/dev/null || true); [[ "$state" != installed ]] || conflicts+=("$package"); done
((${#conflicts[@]}==0)) || fail "Existing conflicting packages (${conflicts[*]}) were preserved. Resolve them manually before opting in; existing workloads are not removed automatically."
"${root[@]}" apt-get update
"${root[@]}" apt-get install -y ca-certificates curl openssl
"${root[@]}" install -m 0755 -d /etc/apt/keyrings
sources=/etc/apt/sources.list.d/docker.sources
if [[ -e "$sources" ]]; then grep -Fq "URIs: https://download.docker.com/linux/$distro" "$sources" && grep -Fq 'Signed-By: /etc/apt/keyrings/docker.asc' "$sources" || fail 'An existing Docker apt source differs; it was not overwritten.'
else
 key=$(mktemp); sourcefile=$(mktemp)
 trap 'rm -f -- "$key" "$sourcefile"' EXIT
 curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location "https://download.docker.com/linux/$distro/gpg" --output "$key"
 grep -q 'BEGIN PGP PUBLIC KEY BLOCK' "$key" || fail 'Official apt key download is invalid'
 "${root[@]}" install -m 0644 "$key" /etc/apt/keyrings/docker.asc
 architecture=$(dpkg --print-architecture)
 printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/docker.asc\n' "$distro" "$suite" "$architecture" > "$sourcefile"
 "${root[@]}" install -m 0644 "$sourcefile" "$sources"
fi
"${root[@]}" apt-get update
"${root[@]}" apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
"${root[@]}" systemctl enable --now docker
"${root[@]}" docker info >/dev/null
printf '%s\n' 'Official Docker Engine and Compose installed. No user was added to the privileged docker group.' 'If your account cannot access Docker, rerun the Painter command through your authorized root/sudo workflow; this script does not change group membership.'
