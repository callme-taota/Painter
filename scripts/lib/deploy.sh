#!/usr/bin/env bash
# Sourced only by project scripts; never source a user's dotenv file.
set -euo pipefail
umask 077
say() { printf '%s\n' "$*"; }
die() { printf 'Painter: %s\n' "$*" >&2; exit 1; }
# Call directly after a failed subprocess: preserve its exit status for automation.
command_failed() { local status=$?; printf 'Painter: %s\n' "$*" >&2; exit "$status"; }
need() { command -v "$1" >/dev/null 2>&1 || die "Required command '$1' is missing."; }
root_dir() { ROOT=$(cd "$1" && pwd -P) || die 'Project directory does not exist.'; [[ -f "$ROOT/compose.yaml" ]] || die 'compose.yaml is missing from project directory.'; }
lock_operation() { LOCK_DIR="$ROOT/.painter-operation.lock"; mkdir "$LOCK_DIR" 2>/dev/null || die "Another operation is running, or a stale lock remains: $LOCK_DIR. Check its pid before removing that directory."; printf '%s\n' "$$" > "$LOCK_DIR/pid"; trap cleanup_operation EXIT; trap 'exit 130' INT; trap 'exit 143' TERM; }
cleanup_operation() { if [[ -n "${ENV_TEMP:-}" && -f "$ENV_TEMP" ]]; then rm -f -- "$ENV_TEMP"; fi; if [[ -n "${LOCK_DIR:-}" && -d "$LOCK_DIR" ]]; then rm -f -- "$LOCK_DIR/pid"; rmdir -- "$LOCK_DIR" 2>/dev/null || true; fi; }
# Read a selected literal value, without running shell expansion. Generated files
# use single quotes. Other dotenv keys/expressions are never executed or exported.
env_value() { local key=$1 line value='' found=0; [[ -f "$ROOT/.env" && ! -L "$ROOT/.env" ]] || return 0; while IFS= read -r line || [[ -n "$line" ]]; do case "$line" in "$key="*) ((found+=1)); value=${line#*=};; esac; done < "$ROOT/.env"; ((found<=1)) || die "Duplicate $key in .env; resolve it explicitly."; if [[ "$value" == \'*\' ]]; then value=${value:1:${#value}-2}; value=${value//\\\'/\'}; elif [[ "$value" == \"*\" ]]; then value=${value:1:${#value}-2}; fi; printf '%s' "$value"; }
env_line() { local value=$2; [[ "$value" != *$'\n'* && "$value" != *$'\r'* ]] || die 'Configuration values must be single-line.'; value=${value//\'/\\\'}; printf "%s='%s'\n" "$1" "$value"; }
valid_port() { [[ "$1" =~ ^[0-9]{1,5}$ ]] && ((10#$1>=1 && 10#$1<=65535)); }
valid_project() { [[ "$1" =~ ^[a-z0-9][a-z0-9_-]{0,39}$ ]]; }
canonical_lan_ip() { local ip=$1 a b c d extra; IFS=. read -r a b c d extra <<< "$ip"; [[ -z "${extra:-}" ]] || return 1; for v in "${a:-}" "${b:-}" "${c:-}" "${d:-}"; do [[ "$v" =~ ^[0-9]{1,3}$ ]] && ((10#$v<=255)) || return 1; done; a=$((10#$a)); b=$((10#$b)); c=$((10#$c)); d=$((10#$d)); ((a==10 || (a==172 && b>=16 && b<=31) || (a==192 && b==168))) || return 1; ((d!=0 && d!=255)) || return 1; printf '%d.%d.%d.%d' "$a" "$b" "$c" "$d"; }
detect_lan_ip() { local candidate; if command -v ip >/dev/null 2>&1; then candidate=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++)if($i=="src"){print $(i+1);exit}}'); if [[ -n "$candidate" ]] && canonical_lan_ip "$candidate" 2>/dev/null; then return; fi; fi; if command -v hostname >/dev/null 2>&1; then for candidate in $(hostname -I 2>/dev/null || true); do if canonical_lan_ip "$candidate" 2>/dev/null; then return; fi; done; fi; return 1; }
valid_domain() { local label; [[ "$1" == *.* && ${#1} -le 253 && "$1" != *..* && "$1" != *[!a-z0-9.-]* ]] || return 1; IFS=. read -ra labels <<< "$1"; for label in "${labels[@]}"; do [[ "$label" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ ]] || return 1; done; [[ ! "$1" =~ ^[0-9.]+$ ]]; }
valid_image() {
 local ref=$1 named last tag='' first part; local -a pieces
 [[ "$ref" != -* && "$ref" != *://* && "$ref" != *//* && "$ref" != */ && "$ref" =~ ^[a-zA-Z0-9][a-zA-Z0-9./:_@-]+$ ]] || return 1
 named=$ref
 if [[ "$ref" == *@* ]]; then [[ "$ref" =~ @sha256:[0-9a-f]{64}$ ]] || return 1; named=${ref%@*}; fi
 last=${named##*/}
 if [[ "$last" == *:* ]]; then tag=${last##*:}; [[ "$tag" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ && "$tag" != latest ]] || return 1; named=${named%:*}; elif [[ "$ref" != *@* ]]; then return 1; fi
 [[ -n "$named" && ${#named} -le 255 ]] || return 1
 IFS=/ read -ra pieces <<< "$named"
 for part in "${pieces[@]}"; do
  if [[ "$part" == *:* && "$part" == "${pieces[0]}" && ${#pieces[@]} -gt 1 ]]; then [[ "$part" =~ ^[a-z0-9.-]+:[0-9]{1,5}$ ]] || return 1
  else [[ "$part" =~ ^[a-z0-9]+(([._]|__|[-]+)[a-z0-9]+)*$ ]] || return 1; fi
 done
 return 0
}
parse_options() {
 unset EFFECTIVE_IMAGE
 ROOT_REQUEST=$DEFAULT_ROOT; MODE=''; LAN_IP=''; DOMAIN=''; PORT=8080; PROJECT_REQUEST=''; IMAGE=''; BUILD=0; NONINTERACTIVE=0; INSTALL_DOCKER=0; TIMEOUT=180; LOG_TAIL=100; LOG_FOLLOW=0; CLIENT_ID=''; SECRET_FILE=''; OWNER_ID=''; SECRET=''; MODE_SET=0; IMAGE_SET=0; BACKUP_NAME=''
 while (($#)); do case "$1" in
 --root) [[ $# -ge 2 ]] || die '--root needs a directory'; ROOT_REQUEST=$2; shift 2;;
 --mode) [[ $# -ge 2 ]] || die '--mode needs local, lan or https'; MODE=$2; MODE_SET=1; shift 2;;
 --local) MODE=local; MODE_SET=1; shift;;
 --lan|--lan-ip) [[ $# -ge 2 ]] || die "$1 needs a private IPv4"; MODE=lan; MODE_SET=1; LAN_IP=$2; shift 2;;
 --domain) [[ $# -ge 2 ]] || die '--domain needs a hostname'; MODE=https; MODE_SET=1; DOMAIN=$(printf '%s' "$2" | tr '[:upper:]' '[:lower:]'); shift 2;;
 --port) [[ $# -ge 2 ]] || die '--port needs a number'; PORT=$2; shift 2;;
 --project) [[ $# -ge 2 ]] || die '--project needs a name'; PROJECT_REQUEST=$2; shift 2;;
 --image) [[ $# -ge 2 ]] || die '--image needs a tagged image or digest'; IMAGE=$2; IMAGE_SET=1; shift 2;;
 --build) BUILD=1; shift;;
 --github-client-id) [[ $# -ge 2 ]] || die '--github-client-id needs a value'; CLIENT_ID=$2; shift 2;;
 --github-client-secret-file) [[ $# -ge 2 ]] || die '--github-client-secret-file needs a path'; SECRET_FILE=$2; shift 2;;
 --owner-github-id|--bootstrap-github-id) [[ $# -ge 2 ]] || die "$1 needs a numeric GitHub ID"; OWNER_ID=$2; shift 2;;
 --non-interactive|--yes) NONINTERACTIVE=1; shift;;
 --install-docker) INSTALL_DOCKER=1; shift;;
 --timeout) [[ $# -ge 2 ]] || die '--timeout needs seconds'; TIMEOUT=$2; shift 2;;
 --tail) [[ $# -ge 2 ]] || die '--tail needs a count'; LOG_TAIL=$2; shift 2;;
 --follow|-f) LOG_FOLLOW=1; shift;;
 --backup) [[ $# -ge 2 ]] || die '--backup needs a directory name from backups'; BACKUP_NAME=$2; shift 2;;
 *) die "Unknown option: $1";; esac; done
 [[ "$TIMEOUT" =~ ^[0-9]{1,5}$ ]] && ((10#$TIMEOUT>=1 && 10#$TIMEOUT<=3600)) || die '--timeout must be 1..3600 seconds'
 [[ "$LOG_TAIL" =~ ^[0-9]{1,5}$ ]] || die '--tail must be 0..99999'
 valid_port "$PORT" || die '--port must be 1..65535'; PORT=$((10#$PORT))
 [[ -z "$PROJECT_REQUEST" ]] || valid_project "$PROJECT_REQUEST" || die 'Invalid project name (lowercase letters, digits, underscore, hyphen).'
 [[ -z "$MODE" || "$MODE" == local || "$MODE" == lan || "$MODE" == https ]] || die '--mode must be local, lan or https'
 ((!(BUILD && IMAGE_SET))) || die '--image and --build are mutually exclusive'
 ((IMAGE_SET==0)) || [[ -n "$IMAGE" ]] || die '--image cannot be empty'
 [[ -z "$IMAGE" ]] || valid_image "$IMAGE" || die '--image requires an explicit non-latest tag or sha256 digest, without shell/URL credentials.'
 [[ -z "$BACKUP_NAME" || "$BACKUP_NAME" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$ ]] || die '--backup must be one directory name, without paths or shell characters.'
 root_dir "$ROOT_REQUEST"
}
validate_command_options() {
 local verb=$1
 if [[ "$verb" != install && "$verb" != update ]]; then ((IMAGE_SET==0 && BUILD==0)) || die '--image and --build apply only to install/update.'; fi
 if [[ "$verb" != install ]]; then [[ "$MODE_SET" == 0 && -z "$PROJECT_REQUEST$CLIENT_ID$SECRET_FILE$OWNER_ID" && "$INSTALL_DOCKER" == 0 ]] || die 'Access, project, OAuth and Docker-install options apply only to install.'; fi
 if [[ "$verb" == verify-backup ]]; then [[ -n "$BACKUP_NAME" ]] || die 'verify-backup requires --backup DIRECTORY_NAME (see backups).'; else [[ -z "$BACKUP_NAME" ]] || die '--backup applies only to verify-backup.'; fi
}
ask() { local label=$1 default=$2 answer; printf '%s [%s]: ' "$label" "$default" >&2; IFS= read -r answer || die 'Input ended. Use --non-interactive with explicit options.'; printf '%s' "${answer:-$default}"; }
new_configuration() {
 need openssl
 if ((NONINTERACTIVE==0)) && [[ -t 0 ]]; then
  say 'Painter installation: choose access → optional GitHub sign-in → start. Local accounts are the default login.' >&2
  if [[ -z "$MODE" ]]; then MODE=$(ask 'Access mode: local, lan, https' lan); fi
  if [[ "$MODE" == lan && -z "$LAN_IP" ]]; then LAN_IP=$(ask 'Private LAN IPv4 (bind only this address)' "$(detect_lan_ip || true)"); fi
  if [[ "$MODE" == https && -z "$DOMAIN" ]]; then DOMAIN=$(ask 'HTTPS hostname (DNS must already point here)' ''); DOMAIN=$(printf '%s' "$DOMAIN" | tr '[:upper:]' '[:lower:]'); fi
  PORT=$(ask 'Host application port' "$PORT")
  if [[ -z "$CLIENT_ID$SECRET_FILE$OWNER_ID" ]]; then local oauth; oauth=$(ask 'Configure optional GitHub OAuth now? y/N' N); if [[ "$oauth" =~ ^[yY]$ ]]; then CLIENT_ID=$(ask 'GitHub OAuth Client ID' ''); printf 'GitHub OAuth Client secret (hidden): ' >&2; IFS= read -r -s SECRET; printf '\n' >&2; OWNER_ID=$(ask 'First owner immutable numeric GitHub ID' ''); fi; fi
 fi
 if ((IMAGE_SET==0 && BUILD==0)) && [[ -f "$ROOT/deploy/release-image.txt" ]]; then IMAGE=$(cat "$ROOT/deploy/release-image.txt"); valid_image "$IMAGE" || die 'Release bundle image hint is invalid'; fi
 MODE=${MODE:-local}; valid_port "$PORT" || die 'Invalid application port'; PORT=$((10#$PORT)); PROJECT=${PROJECT_REQUEST:-painter-next}; valid_project "$PROJECT" || die 'Invalid project name'
 case "$MODE" in
 local) ENVIRONMENT=development; BIND=127.0.0.1; PUBLIC_HOST=localhost; URL="http://localhost:$PORT";;
 lan) if ((NONINTERACTIVE)) && [[ -z "$LAN_IP" ]]; then die 'Non-interactive LAN mode requires --lan-ip with an explicit private IPv4'; fi; LAN_IP=${LAN_IP:-$(detect_lan_ip || true)}; BIND=$(canonical_lan_ip "$LAN_IP") || die 'LAN mode needs one explicit private IPv4; wildcard/public/IPv6 binding is rejected.'; ENVIRONMENT=development; PUBLIC_HOST=$BIND; URL="http://$BIND:$PORT";;
 https) DOMAIN=$(printf '%s' "$DOMAIN" | tr '[:upper:]' '[:lower:]'); valid_domain "$DOMAIN" || die 'HTTPS mode needs a valid public DNS hostname'; ((PORT!=80 && PORT!=443)) || die 'The application port must leave 80/443 available to Caddy'; ENVIRONMENT=production; BIND=127.0.0.1; PUBLIC_HOST=$DOMAIN; URL="https://$DOMAIN";;
 *) die 'Invalid access mode';; esac
 if [[ -n "$SECRET_FILE" ]]; then [[ -f "$SECRET_FILE" && ! -L "$SECRET_FILE" ]] || die 'GitHub secret file must be a regular file'; [[ $(wc -c < "$SECRET_FILE") -le 4096 ]] || die 'GitHub secret file is too large'; SECRET=$(cat -- "$SECRET_FILE") || die 'Cannot read GitHub secret file'; fi
 [[ "$CLIENT_ID$SECRET$OWNER_ID" != *$'\n'* && "$CLIENT_ID$SECRET$OWNER_ID" != *$'\r'* ]] || die 'GitHub values must be single-line'
 if [[ -n "$CLIENT_ID$SECRET$OWNER_ID" ]]; then [[ -n "$CLIENT_ID" && -n "$SECRET" && "$OWNER_ID" =~ ^[1-9][0-9]{0,17}$ ]] || die 'Provide a real Client ID, secret file and positive numeric owner GitHub ID together'; fi
 local db_admin db_migration db_app app_key analytics_key setup_token
 db_admin=$(openssl rand -hex 32) || die 'Secret generation failed'; db_migration=$(openssl rand -hex 32) || die 'Secret generation failed'; db_app=$(openssl rand -hex 32) || die 'Secret generation failed'; app_key=$(openssl rand -base64 32) || die 'Secret generation failed'; analytics_key=$(openssl rand -base64 32) || die 'Secret generation failed'
 [[ -n "$db_admin" && -n "$db_migration" && -n "$db_app" && -n "$app_key" && -n "$analytics_key" && "$app_key" != "$analytics_key" ]] || die 'Secret generator returned invalid or repeated keys'
 setup_token=$(openssl rand -hex 32) || die 'Installation token generation failed'; [[ "$setup_token" =~ ^[0-9a-f]{64}$ ]] || die 'Invalid installation token'
 ENV_TEMP=$(mktemp "$ROOT/.env.tmp.XXXXXX"); chmod 0600 "$ENV_TEMP"
 { env_line PAINTER_DEPLOY_MODE "$MODE"; env_line COMPOSE_PROJECT_NAME "$PROJECT"; env_line ENVIRONMENT "$ENVIRONMENT"; env_line PUBLIC_BASE_URL "$URL"; env_line PUBLIC_HOST "$PUBLIC_HOST"; env_line APP_PORT "$PORT"; env_line APP_BIND_ADDRESS "$BIND"; env_line PAINTER_IMAGE "$IMAGE"; env_line DB_ADMIN_PASSWORD "$db_admin"; env_line DB_MIGRATION_PASSWORD "$db_migration"; env_line DB_APP_PASSWORD "$db_app"; env_line APP_ENCRYPTION_KEY "$app_key"; env_line ANALYTICS_HMAC_KEY "$analytics_key"; env_line SETUP_TOKEN "$setup_token"; env_line GITHUB_CLIENT_ID "$CLIENT_ID"; env_line GITHUB_CLIENT_SECRET "$SECRET"; env_line BOOTSTRAP_GITHUB_ID "$OWNER_ID"; } > "$ENV_TEMP"
 ln "$ENV_TEMP" "$ROOT/.env" 2>/dev/null || die '.env appeared while configuring; it was preserved.'; rm -f -- "$ENV_TEMP"; ENV_TEMP=''; sync -f "$ROOT/.env" 2>/dev/null || sync; sync -f "$ROOT" 2>/dev/null || sync; mkdir -p "$ROOT/backups"; chmod 0700 "$ROOT/backups"
 say 'Created .env (0600); independent secrets are stored only in that file.'
}
load_configuration() {
 [[ -f "$ROOT/.env" && ! -L "$ROOT/.env" ]] || die 'No regular .env found. Run install or configure.sh first.'
 chmod 0600 "$ROOT/.env" || die 'Cannot protect .env permissions; use its owner account.'
 PROJECT=$(env_value COMPOSE_PROJECT_NAME); PROJECT=${PROJECT:-painter-next}; valid_project "$PROJECT" || die 'Invalid COMPOSE_PROJECT_NAME in .env'; [[ -z "$PROJECT_REQUEST" || "$PROJECT_REQUEST" == "$PROJECT" ]] || die '--project cannot switch an existing deployment to different volumes.'
 URL=$(env_value PUBLIC_BASE_URL); [[ "$URL" =~ ^https?://[a-zA-Z0-9.:-]+/?$ ]] || die 'PUBLIC_BASE_URL must be a literal HTTP(S) origin.'
 CONFIGURED_IMAGE=$(env_value PAINTER_IMAGE); [[ -z "$CONFIGURED_IMAGE" ]] || valid_image "$CONFIGURED_IMAGE" || die 'PAINTER_IMAGE must have an explicit tag or digest.'
 CURRENT_MODE=$(env_value PAINTER_DEPLOY_MODE); if [[ -z "$CURRENT_MODE" ]]; then if [[ "$URL" == https:* ]]; then CURRENT_MODE=https; elif [[ "$(env_value APP_BIND_ADDRESS)" != 127.0.0.1 && -n "$(env_value APP_BIND_ADDRESS)" ]]; then CURRENT_MODE=lan; else CURRENT_MODE=local; fi; fi
 [[ "$CURRENT_MODE" == local || "$CURRENT_MODE" == lan || "$CURRENT_MODE" == https ]] || die 'Invalid PAINTER_DEPLOY_MODE'
 local bind; bind=$(env_value APP_BIND_ADDRESS); bind=${bind:-127.0.0.1}; if [[ "$CURRENT_MODE" == lan ]]; then [[ "$(canonical_lan_ip "$bind" || true)" == "$bind" ]] || die 'LAN configuration must bind one canonical private IPv4'; else [[ "$bind" == 127.0.0.1 ]] || die 'Local/HTTPS application binding must remain 127.0.0.1'; fi
}
# All managed settings come from the literal dotenv file, not the caller's shell.
# Compose handles single-quoted secret escaping; no shell eval/source is involved.
compose() { (
 unset ENVIRONMENT LISTEN_ADDR PUBLIC_BASE_URL MEDIA_DIR APP_ENCRYPTION_KEY ANALYTICS_HMAC_KEY SETUP_TOKEN GITHUB_CLIENT_ID GITHUB_CLIENT_SECRET BOOTSTRAP_GITHUB_ID APP_PORT APP_BIND_ADDRESS PUBLIC_HOST DB_ADMIN_PASSWORD DB_MIGRATION_PASSWORD DB_APP_PASSWORD PAINTER_IMAGE COMPOSE_PROJECT_NAME COMPOSE_PROFILES GOMEMLIMIT GOMAXPROCS
 if [[ "${EFFECTIVE_IMAGE+x}" == x ]]; then export PAINTER_IMAGE=$EFFECTIVE_IMAGE; fi
 local -a args=(compose --project-directory "$ROOT" --env-file "$ROOT/.env" --project-name "$PROJECT" -f "$ROOT/compose.yaml"); [[ "$CURRENT_MODE" != https ]] || args+=(--profile edge)
 docker "${args[@]}" "$@"
 ); }
docker_preflight() {
 if ((INSTALL_DOCKER)); then [[ "$(uname -s)" == Linux ]] || die '--install-docker supports Linux Ubuntu/Debian only'; local -a install_args=(); ((NONINTERACTIVE==0)) || install_args+=(--yes); "$DEFAULT_ROOT/scripts/install-docker.sh" "${install_args[@]}"; fi
 need docker
 local version major minor; version=$(docker compose version --short 2>/dev/null) || command_failed 'Docker Compose plugin is unavailable; install Docker Compose v2.20+ (or newer major).'; version=${version#v}; [[ "$version" =~ ^([0-9]+)\.([0-9]+) ]] || die 'Cannot determine Docker Compose version'; major=${BASH_REMATCH[1]}; minor=${BASH_REMATCH[2]}; ((major>2 || (major==2 && minor>=20))) || die 'Docker Compose v2.20+ is required.'
 docker info >/dev/null 2>&1 || command_failed 'Docker daemon is not reachable for this user. Start Docker or use an authorized account; no configuration or running service was changed.'
}
validate_compose() { compose config --quiet || command_failed 'Compose configuration is invalid; no service was started.'; }
service_id() { compose ps -a -q "$1" | head -n 1; }
wait_migration() {
 local began=$SECONDS id state code
 while ((SECONDS-began<TIMEOUT)); do
  id=$(service_id migrate)
  if [[ -n "$id" ]]; then
   state=$(docker inspect --format '{{.State.Status}}' "$id") || command_failed 'Cannot inspect migration state.'
   if [[ "$state" == exited ]]; then
    code=$(docker inspect --format '{{.State.ExitCode}}' "$id") || command_failed 'Cannot inspect migration exit status.'
    if [[ "$code" != 0 ]]; then
     printf '%s\n' 'Painter: Migration failed. The old application is stopped; database, media and pre-update backup remain. Inspect logs and repair/roll forward or restore into a separate database; do not blindly downgrade schema.' >&2
     [[ "$code" =~ ^[0-9]{1,3}$ ]] && ((10#$code>0 && 10#$code<=255)) || exit 1
     exit "$code"
    fi
    return
   fi
   [[ "$state" != dead ]] || die 'Migration container is dead.'
  fi
  sleep 1
 done
 die 'Migration timed out; app activation was not attempted. Inspect logs and the backup before retrying.'
}
wait_health() {
 local began=$SECONDS id health state
 while ((SECONDS-began<TIMEOUT)); do
  id=$(service_id app)
  if [[ -n "$id" ]]; then
   health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id") || command_failed 'Cannot inspect application health.'
   [[ "$health" != healthy ]] || return 0
   [[ "$health" != unhealthy ]] || die 'Application healthcheck failed. Keep the backup and inspect logs before a controlled recovery.'
   state=$(docker inspect --format '{{.State.Status}}' "$id") || command_failed 'Cannot inspect application state.'
   [[ "$state" != exited && "$state" != dead ]] || die 'Application exited before becoming healthy.'
  fi
  sleep 1
 done
 die 'Application health timed out. No success was recorded; data and backups remain in place.'
}

backup_site() {
 local name
 need openssl
 name="$(date -u +%Y%m%dT%H%M%SZ)-$(openssl rand -hex 4)"
 BACKUP_CONTAINER_DIR="/var/backups/painter/$name"
 [[ ! -L "$ROOT/backups" ]] || die 'backups must not be a symbolic link.'
 mkdir -p "$ROOT/backups"; chmod 0700 "$ROOT/backups"
 say "Backup: $BACKUP_CONTAINER_DIR"
 compose exec -T app painter backup "$BACKUP_CONTAINER_DIR" || command_failed 'Backup failed; update was not started.'
 compose cp "app:$BACKUP_CONTAINER_DIR" "$ROOT/backups/" || command_failed 'Backup export failed; update was not started. The container backup was retained.'
 LAST_BACKUP="$ROOT/backups/$name"
 [[ -f "$LAST_BACKUP/manifest.json" && -f "$LAST_BACKUP/database.dump" && ! -L "$LAST_BACKUP" ]] || die 'Exported backup manifest/database archive is missing; update was not started.'
 chmod 0700 "$LAST_BACKUP"
 # The original encryption/OAuth keys are necessary for full disaster recovery.
 # They remain private on this host; the public release bundle never includes them.
 cp -- "$ROOT/.env" "$LAST_BACKUP/deployment.env" || command_failed 'Configuration backup failed; update was not started.'
 chmod 0600 "$LAST_BACKUP/deployment.env"
 cp -- "$ROOT/compose.yaml" "$LAST_BACKUP/compose.yaml" || command_failed 'Compose snapshot failed; update was not started.'
 printf 'Project: %s\nURL: %s\nMode: %s\nImage: %s\n' "$PROJECT" "$URL" "$CURRENT_MODE" "${CONFIGURED_IMAGE:-local source build}" > "$LAST_BACKUP/deployment.txt"
 say "Backup exported: $LAST_BACKUP"
 say 'Private deployment.env included. Recovery has not yet been verified; use verify-backup.'
}
list_backups() {
 [[ ! -L "$ROOT/backups" ]] || die 'backups must not be a symbolic link.'
 local path count=0
 say 'Host backups (directory names; recovery validation is a separate operation):'
 if [[ -d "$ROOT/backups" ]]; then
  shopt -s nullglob
  for path in "$ROOT"/backups/*; do
   [[ -d "$path" && ! -L "$path" && -f "$path/manifest.json" ]] || continue
   [[ "${path##*/}" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$ ]] || continue
   printf '  %s' "${path##*/}"
   if [[ -f "$path/deployment.env" ]]; then printf '  [private configuration included]'; fi
   printf '\n'; ((count+=1))
  done
  shopt -u nullglob
 fi
 ((count>0)) || say '  No exported backups found.'
}
verify_backup() {
 local directory="/var/backups/painter/$BACKUP_NAME"
 say "Checking backup in an isolated temporary database: $BACKUP_NAME"
 # This is a one-off ops container with the maintenance DB role, not the app role.
 # --no-deps prevents this check from implicitly starting/recreating live services.
 compose run --rm --no-deps -T ops restore-check "$directory" || command_failed 'Backup verification failed; the live database was not replaced. Inspect the restore report/logs.'
 say 'Backup restored and verified; temporary database cleanup completed.'
}
wait_database() {
 local began=$SECONDS id state health
 while ((SECONDS-began<TIMEOUT)); do
  id=$(service_id db)
  [[ -n "$id" ]] || die 'Existing database container is missing; use install rather than start.'
  state=$(docker inspect --format '{{.State.Status}}' "$id") || command_failed 'Cannot inspect database state.'
  health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id") || command_failed 'Cannot inspect database health.'
  if [[ "$state" == running && "$health" == healthy ]]; then return; fi
  [[ "$state" != exited && "$state" != dead && "$health" != unhealthy ]] || die 'Database is not healthy; the application was not started.'
  sleep 1
 done
 die 'Database health timed out; the application was not started.'
}
start_site() {
 local service; local -a services=(app)
 [[ "$CURRENT_MODE" != https ]] || services+=(caddy)
 for service in db "${services[@]}"; do [[ -n "$(service_id "$service")" ]] || die "Existing $service container is missing; use install rather than start."; done
 compose start db || command_failed 'Database start failed.'
 wait_database
 compose start "${services[@]}" || command_failed 'Application start failed.'
 wait_health
}
doctor_site() {
 local service id state health failed=0; local -a services=(db app)
 [[ "$CURRENT_MODE" != https ]] || services+=(caddy)
 say "Project: $PROJECT"; say "URL: $URL"; say "Mode: $CURRENT_MODE"
 for service in "${services[@]}"; do
  id=$(service_id "$service") || command_failed 'Cannot list deployment containers.'
  if [[ -z "$id" ]]; then say "FAIL $service: container missing"; failed=1; continue; fi
  state=$(docker inspect --format '{{.State.Status}}' "$id") || command_failed 'Cannot inspect service state.'
  if [[ "$service" == caddy ]]; then
   if [[ "$state" == running ]]; then say 'OK   caddy: running (external DNS/TLS not checked)'; else say "FAIL caddy: $state"; failed=1; fi
  else
   health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id") || command_failed 'Cannot inspect service health.'
   if [[ "$state" == running && "$health" == healthy ]]; then say "OK   $service: running / healthy"; else say "FAIL $service: $state / $health"; failed=1; fi
  fi
 done
 # Perform a fresh HTTP readiness check rather than relying only on the last
 # container health poll. /readyz verifies DB/schema/projection readiness.
 if ((failed==0)); then compose exec -T app painter healthcheck || command_failed 'Fresh application readiness check failed.'; say 'OK   fresh application readiness'; fi
 say 'Host filesystem capacity (not container RSS or a resource budget measurement):'
 df -Pk "$ROOT" || command_failed 'Cannot read host filesystem capacity.'
 list_backups
 ((failed==0)) || die 'Deployment checks failed. Use logs; no services or volumes were changed.'
 say 'Deployment checks passed. External network/firewall/TLS reachability requires a client on that network.'
}

activate() {
 if [[ -n "${EFFECTIVE_IMAGE:-}" ]]; then say 'Pulling the explicitly configured release image; no Node/Go compilation on this server.'; compose pull app db || command_failed 'Image pull failed; the running application was not replaced.'
 else [[ -f "$ROOT/Dockerfile" && -f "$ROOT/go.mod" && -f "$ROOT/web/frontend/package-lock.json" ]] || die 'Source build requires the full checkout. Use --image with a real published release image for a deployment bundle.'; say 'Building local source (Node/Go build RAM is separate from the 1 GiB runtime target).'; compose build app || command_failed 'Source build failed; the running application was not replaced.'; fi
 say 'Stopping the previous application before schema changes (data and volumes retained)...'
 compose stop --timeout 30 app || command_failed 'Could not stop the old application; migration was not started.'
 say 'Starting database and explicitly rerunning the new migration container...'
 compose up --no-build -d --force-recreate migrate || command_failed 'Could not start the new migration; inspect logs before retrying.'
 wait_migration
 say 'Migration completed; activating the application...'
 local -a services=(app); [[ "$CURRENT_MODE" != https ]] || services+=(caddy)
 compose up --no-build -d "${services[@]}" || command_failed 'Application activation failed after migration. Keep the backup; do not blindly downgrade schema or delete volumes.'
 wait_health
 if [[ "$CURRENT_MODE" == https ]]; then local edge; edge=$(service_id caddy); [[ -n "$edge" && "$(docker inspect --format '{{.State.Status}}' "$edge")" == running ]] || die 'Application is healthy but Caddy is not running; inspect edge logs.'; fi
}
persist_image() { local value=$1 line; ENV_TEMP=$(mktemp "$ROOT/.env.tmp.XXXXXX"); chmod 0600 "$ENV_TEMP"; while IFS= read -r line || [[ -n "$line" ]]; do [[ "$line" == PAINTER_IMAGE=* ]] || printf '%s\n' "$line"; done < "$ROOT/.env" > "$ENV_TEMP"; env_line PAINTER_IMAGE "$value" >> "$ENV_TEMP"; mv -f -- "$ENV_TEMP" "$ROOT/.env"; ENV_TEMP=''; sync -f "$ROOT/.env" 2>/dev/null || sync; sync -f "$ROOT" 2>/dev/null || sync; }
print_access() {
 if [[ "$CURRENT_MODE" == https ]]; then say "Application healthy; HTTPS entry configured: $URL"; else say "Ready: $URL"; fi
 say "Local account login: $URL/login"
 say "First installation: $URL/setup"
 say 'Read the one-time installation code on this server: ./scripts/painter setup-token'
 say 'No default account or password is created. The web wizard creates the first owner; offline painter create-local-user CLI remains available. Existing sites cannot be reinstalled. See docs/DEPLOYMENT.md.'
 say 'Production registration is closed by default; an owner can enable registration_enabled in administration.'
 say "GitHub OAuth is optional. Callback: $URL/auth/github/callback"
 if [[ "$CURRENT_MODE" == lan ]]; then say 'LAN mode binds only the selected IPv4. Other devices must be on the reachable LAN; localhost cookies/drafts are a different origin.'; elif [[ "$CURRENT_MODE" == https ]]; then say 'Caddy is running; public DNS, port forwarding and ACME issuance must also succeed for external HTTPS access.'; fi
}
