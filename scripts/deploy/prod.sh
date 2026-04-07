#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

IMAGE_NAME_DEFAULT="mrsmith:prod"
IMAGE_ARCHIVE_TAG_DEFAULT="mrsmith:amd64"
DEPLOY_PORT_DEFAULT="22"
DEPLOY_REMOTE_DIR_DEFAULT="/root"
DEPLOY_COMPOSE_FILE_DEFAULT="/DATI/mrsmith/docker-compose.yaml"
DEPLOY_SERVICE_DEFAULT="mrsmith"
DEPLOY_RETENTION_DEFAULT="5"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/deploy/prod.sh build
  ./scripts/deploy/prod.sh package [--build]
  ./scripts/deploy/prod.sh deploy [--build]
  ./scripts/deploy/prod.sh rollback

Environment:
  DEPLOY_ENV_FILE      Path del file env di deploy (default: .env.deploy.prod)
  RELEASE_TS           Timestamp release; se assente usa UTC YYYYmmddHHMMSS
  DRY_RUN=1            Stampa i comandi senza eseguirli
EOF
}

log() {
  printf '[prod-deploy] %s\n' "$*"
}

fail() {
  printf '[prod-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

run_cmd() {
  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    printf '[dry-run]'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi

  "$@"
}

run_remote_script() {
  local script="$1"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    log "DRY_RUN: salterei lo script remoto su ${DEPLOY_USER}@${DEPLOY_HOST}"
    printf '%s\n' "$script"
    return 0
  fi

  ssh -p "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" /bin/sh -s -- \
    "${REMOTE_TAR_PATH}" \
    "${IMAGE_NAME}" \
    "${DEPLOY_COMPOSE_FILE}" \
    "${DEPLOY_SERVICE}" \
    "${DEPLOY_RETENTION}" \
    "${DEPLOY_REMOTE_DIR}" <<EOF
${script}
EOF
}

load_env() {
  DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-${REPO_ROOT}/.env.deploy.prod}"

  if [[ -f "${DEPLOY_ENV_FILE}" ]]; then
    log "Carico configurazione da ${DEPLOY_ENV_FILE}"
    set -a
    # shellcheck disable=SC1090
    source "${DEPLOY_ENV_FILE}"
    set +a
  fi

  IMAGE_NAME="${IMAGE_NAME:-${IMAGE_NAME_DEFAULT}}"
  IMAGE_ARCHIVE_TAG="${IMAGE_ARCHIVE_TAG:-${IMAGE_ARCHIVE_TAG_DEFAULT}}"
  ARTIFACT_DIR="${ARTIFACT_DIR:-${REPO_ROOT}/artifacts/releases}"
  DEPLOY_PORT="${DEPLOY_PORT:-${DEPLOY_PORT_DEFAULT}}"
  DEPLOY_REMOTE_DIR="${DEPLOY_REMOTE_DIR:-${DEPLOY_REMOTE_DIR_DEFAULT}}"
  DEPLOY_COMPOSE_FILE="${DEPLOY_COMPOSE_FILE:-${DEPLOY_COMPOSE_FILE_DEFAULT}}"
  DEPLOY_SERVICE="${DEPLOY_SERVICE:-${DEPLOY_SERVICE_DEFAULT}}"
  DEPLOY_RETENTION="${DEPLOY_RETENTION:-${DEPLOY_RETENTION_DEFAULT}}"
}

set_release_vars() {
  RELEASE_TS="${RELEASE_TS:-$(date -u +%Y%m%d%H%M%S)}"
  RELEASE_FILENAME="mrsmith_amd64_${RELEASE_TS}.tar"
  ARTIFACT_PATH="${ARTIFACT_DIR}/${RELEASE_FILENAME}"
  REMOTE_TAR_PATH="${DEPLOY_REMOTE_DIR%/}/${RELEASE_FILENAME}"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Comando richiesto non trovato: $1"
}

require_local_image() {
  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    return 0
  fi

  docker image inspect "${IMAGE_NAME}" >/dev/null 2>&1 || fail "Immagine locale ${IMAGE_NAME} non trovata. Esegui prima il build."
}

require_deploy_config() {
  [[ -n "${DEPLOY_HOST:-}" ]] || fail "DEPLOY_HOST non impostato"
  [[ -n "${DEPLOY_USER:-}" ]] || fail "DEPLOY_USER non impostato"
}

build_image() {
  require_command docker
  log "Build immagine linux/amd64 con tag ${IMAGE_NAME} e ${IMAGE_ARCHIVE_TAG}"
  run_cmd docker buildx build \
    --platform linux/amd64 \
    -f "${REPO_ROOT}/deploy/Dockerfile" \
    -t "${IMAGE_NAME}" \
    -t "${IMAGE_ARCHIVE_TAG}" \
    --load \
    "${REPO_ROOT}"
}

package_image() {
  require_command docker
  require_local_image

  log "Esporto ${IMAGE_NAME} in ${ARTIFACT_PATH}"
  run_cmd mkdir -p "${ARTIFACT_DIR}"
  run_cmd docker image save -o "${ARTIFACT_PATH}" "${IMAGE_NAME}"
  log "Artefatto pronto: ${ARTIFACT_PATH}"
}

copy_artifact() {
  require_command scp
  require_deploy_config

  log "Copio ${ARTIFACT_PATH} su ${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_TAR_PATH}"
  run_cmd scp -P "${DEPLOY_PORT}" "${ARTIFACT_PATH}" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_TAR_PATH}"
}

deploy_remote() {
  require_command ssh
  require_deploy_config

  log "Carico l'immagine sul server e ricreo il servizio ${DEPLOY_SERVICE}"
  run_remote_script 'set -eu
remote_tar="$1"
image_name="$2"
compose_file="$3"
service="$4"
retention="$5"
remote_dir="$6"

[ -f "$remote_tar" ] || {
  echo "release non trovata: $remote_tar" >&2
  exit 1
}

[ -f "$compose_file" ] || {
  echo "compose file non trovato: $compose_file" >&2
  exit 1
}

docker image load -i "$remote_tar" >/dev/null
docker image inspect "$image_name" >/dev/null
docker compose -f "$compose_file" up -d --force-recreate --no-deps "$service" >/dev/null

container_id="$(docker compose -f "$compose_file" ps -q "$service")"
[ -n "$container_id" ] || {
  echo "servizio $service senza container" >&2
  exit 1
}

status="$(docker inspect --format "{{.State.Status}}" "$container_id")"
[ "$status" = "running" ] || {
  echo "container $container_id non in esecuzione: $status" >&2
  exit 1
}

docker inspect --format "container={{.Name}} image={{.Config.Image}} status={{.State.Status}}" "$container_id"

count=0
for path in $(ls -1t "$remote_dir"/mrsmith_amd64_*.tar 2>/dev/null || true); do
  count=$((count + 1))
  if [ "$count" -gt "$retention" ]; then
    rm -f "$path"
  fi
done'
}

rollback_remote() {
  require_command ssh
  require_deploy_config

  log "Rollback release ${RELEASE_TS} da ${REMOTE_TAR_PATH}"
  run_remote_script 'set -eu
remote_tar="$1"
image_name="$2"
compose_file="$3"
service="$4"
retention="$5"
remote_dir="$6"

[ -f "$remote_tar" ] || {
  echo "release non trovata: $remote_tar" >&2
  exit 1
}

[ -f "$compose_file" ] || {
  echo "compose file non trovato: $compose_file" >&2
  exit 1
}

docker image load -i "$remote_tar" >/dev/null
docker image inspect "$image_name" >/dev/null
docker compose -f "$compose_file" up -d --force-recreate --no-deps "$service" >/dev/null

container_id="$(docker compose -f "$compose_file" ps -q "$service")"
[ -n "$container_id" ] || {
  echo "servizio $service senza container" >&2
  exit 1
}

status="$(docker inspect --format "{{.State.Status}}" "$container_id")"
[ "$status" = "running" ] || {
  echo "container $container_id non in esecuzione: $status" >&2
  exit 1
}

docker inspect --format "container={{.Name}} image={{.Config.Image}} status={{.State.Status}}" "$container_id"'
}

main() {
  load_env

  local subcommand="${1:-}"
  shift || true

  local should_build="0"
  while (($# > 0)); do
    case "$1" in
      --build)
        should_build="1"
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "Argomento non riconosciuto: $1"
        ;;
    esac
    shift
  done

  case "${subcommand}" in
    build)
      build_image
      ;;
    package)
      set_release_vars
      [[ "${should_build}" == "1" ]] && build_image
      package_image
      ;;
    deploy)
      set_release_vars
      [[ "${should_build}" == "1" ]] && build_image
      package_image
      copy_artifact
      deploy_remote
      ;;
    rollback)
      [[ -n "${RELEASE_TS:-}" ]] || fail "Imposta RELEASE_TS=YYYYmmddHHMMSS per il rollback"
      set_release_vars
      rollback_remote
      ;;
    -h|--help|help|"")
      usage
      ;;
    *)
      fail "Sottocomando non riconosciuto: ${subcommand}"
      ;;
  esac
}

main "$@"
