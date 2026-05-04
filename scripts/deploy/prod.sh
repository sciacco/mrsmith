#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

IMAGE_NAME_DEFAULT="mrsmith:prod"
IMAGE_ARCHIVE_TAG_DEFAULT="mrsmith:amd64"
DEPLOY_PORT_DEFAULT="22"
DEPLOY_COMPOSE_FILE_DEFAULT="/DATI/mrsmith/docker-compose.yaml"
DEPLOY_SERVICE_DEFAULT="mrsmith"
DEPLOY_RETENTION_DEFAULT="5"
DEPLOY_SOURCE_REF_DEFAULT="HEAD"
DEPLOY_REQUIRE_CLEAN_DEFAULT="1"

DOCKER_CONTEXT_PATHS=(
  "deploy/Dockerfile"
  "package.json"
  "pnpm-workspace.yaml"
  "pnpm-lock.yaml"
  "tsconfig.base.json"
  "packages"
  "apps"
  "backend"
)

usage() {
  cat <<'EOF'
Usage:
  ./scripts/deploy/prod.sh build
  ./scripts/deploy/prod.sh package [--build]
  ./scripts/deploy/prod.sh deploy
  ./scripts/deploy/prod.sh rollback

Environment:
  DEPLOY_ENV_FILE      Path del file env di deploy (default: .env.deploy.prod)
  DEPLOY_SOURCE_REF    Ref Git da archiviare per il deploy remoto (default: HEAD)
  DEPLOY_REQUIRE_CLEAN Richiede worktree pulito per deploy reali (default: 1)
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
  shift

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    log "DRY_RUN: salterei lo script remoto su ${DEPLOY_USER}@${DEPLOY_HOST}"
    printf '[dry-run] ssh -p %q %q /bin/sh -s --' "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}"
    local arg
    for arg in "$@"; do
      printf ' %q' "${arg}"
    done
    printf '\n'
    printf '%s\n' "$script"
    return 0
  fi

  ssh -p "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" /bin/sh -s -- "$@" <<EOF
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
  DEPLOY_COMPOSE_FILE="${DEPLOY_COMPOSE_FILE:-${DEPLOY_COMPOSE_FILE_DEFAULT}}"
  DEPLOY_SERVICE="${DEPLOY_SERVICE:-${DEPLOY_SERVICE_DEFAULT}}"
  DEPLOY_RETENTION="${DEPLOY_RETENTION:-${DEPLOY_RETENTION_DEFAULT}}"
  DEPLOY_SOURCE_REF="${DEPLOY_SOURCE_REF:-${DEPLOY_SOURCE_REF_DEFAULT}}"
  DEPLOY_REQUIRE_CLEAN="${DEPLOY_REQUIRE_CLEAN:-${DEPLOY_REQUIRE_CLEAN_DEFAULT}}"
  IMAGE_RELEASE_TAG_PREFIX="${IMAGE_RELEASE_TAG_PREFIX:-${IMAGE_NAME}-}"
}

set_release_vars() {
  RELEASE_TS="${RELEASE_TS:-$(date -u +%Y%m%d%H%M%S)}"
  RELEASE_FILENAME="mrsmith_amd64_${RELEASE_TS}.tar"
  ARTIFACT_PATH="${ARTIFACT_DIR}/${RELEASE_FILENAME}"
  RELEASE_IMAGE_TAG="${IMAGE_RELEASE_TAG_PREFIX}${RELEASE_TS}"
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

require_clean_source() {
  if [[ "${DRY_RUN:-0}" == "1" || "${DEPLOY_REQUIRE_CLEAN}" != "1" ]]; then
    return 0
  fi

  git -C "${REPO_ROOT}" diff --quiet -- || fail "Worktree con modifiche non staged: commit o imposta DEPLOY_REQUIRE_CLEAN=0"
  git -C "${REPO_ROOT}" diff --cached --quiet -- || fail "Index con modifiche staged: commit o imposta DEPLOY_REQUIRE_CLEAN=0"

  local untracked
  untracked="$(git -C "${REPO_ROOT}" ls-files --others --exclude-standard)"
  [[ -z "${untracked}" ]] || fail "File untracked non inclusi nel deploy da commit: commit, ignora o imposta DEPLOY_REQUIRE_CLEAN=0"
}

resolve_source_revision() {
  require_command git

  if ! SOURCE_REV="$(git -C "${REPO_ROOT}" rev-parse --verify "${DEPLOY_SOURCE_REF}^{commit}")"; then
    fail "DEPLOY_SOURCE_REF non risolve a un commit: ${DEPLOY_SOURCE_REF}"
  fi

  SOURCE_SHORT_REV="$(git -C "${REPO_ROOT}" rev-parse --short "${SOURCE_REV}")"
}

shell_quote() {
  local value="$1"
  [[ "${value}" != *"'"* ]] || fail "Valore remoto non supportato perche contiene apici singoli: ${value}"
  printf "'%s'" "${value}"
}

check_remote_tools() {
  require_command ssh
  require_deploy_config

  log "Verifico strumenti Docker sul server produzione"
  run_remote_script 'set -eu
command -v docker >/dev/null 2>&1 || {
  echo "docker non trovato sul server" >&2
  exit 1
}
docker buildx version >/dev/null
docker compose version >/dev/null'
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

build_remote_image_from_stream() {
  require_command git
  require_command ssh
  require_deploy_config
  require_clean_source
  [[ -n "${SOURCE_REV:-}" ]] || resolve_source_revision

  local remote_build_cmd
  remote_build_cmd="$(cat <<EOF
set -eu
image_name=$(shell_quote "${IMAGE_NAME}")
archive_tag=$(shell_quote "${IMAGE_ARCHIVE_TAG}")
release_tag=$(shell_quote "${RELEASE_IMAGE_TAG}")
release_ts=$(shell_quote "${RELEASE_TS}")
source_rev=$(shell_quote "${SOURCE_REV}")
source_ref=$(shell_quote "${DEPLOY_SOURCE_REF}")
docker buildx build \\
  --platform linux/amd64 \\
  -f deploy/Dockerfile \\
  -t "\$image_name" \\
  -t "\$archive_tag" \\
  -t "\$release_tag" \\
  --label "org.opencontainers.image.revision=\$source_rev" \\
  --label "org.opencontainers.image.ref.name=\$source_ref" \\
  --label "com.mrsmith.release.timestamp=\$release_ts" \\
  --load \\
  -
docker image inspect "\$image_name" >/dev/null
docker image inspect "\$release_tag" >/dev/null
EOF
)"

  log "Stream sorgente ${DEPLOY_SOURCE_REF} (${SOURCE_SHORT_REV}) e build remoto ${RELEASE_IMAGE_TAG}"

  if [[ "${DRY_RUN:-0}" == "1" ]]; then
    printf '[dry-run] git -C %q archive --format=tar %q --' "${REPO_ROOT}" "${SOURCE_REV}"
    local path
    for path in "${DOCKER_CONTEXT_PATHS[@]}"; do
      printf ' %q' "${path}"
    done
    printf ' | ssh -p %q %q %q\n' "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" "${remote_build_cmd}"
    return 0
  fi

  git -C "${REPO_ROOT}" archive --format=tar "${SOURCE_REV}" -- "${DOCKER_CONTEXT_PATHS[@]}" |
    ssh -p "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" "${remote_build_cmd}"
}

restart_remote_service() {
  require_command ssh
  require_deploy_config

  log "Ricreo il servizio ${DEPLOY_SERVICE} con immagine ${IMAGE_NAME}"
  run_remote_script 'set -eu
image_name="$1"
compose_file="$2"
service="$3"

[ -f "$compose_file" ] || {
  echo "compose file non trovato: $compose_file" >&2
  exit 1
}

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

docker inspect --format "container={{.Name}} image={{.Config.Image}} status={{.State.Status}}" "$container_id"' \
    "${IMAGE_NAME}" \
    "${DEPLOY_COMPOSE_FILE}" \
    "${DEPLOY_SERVICE}"
}

prune_remote_release_tags() {
  require_command ssh
  require_deploy_config

  log "Mantengo le ultime ${DEPLOY_RETENTION} release image tag con prefisso ${IMAGE_RELEASE_TAG_PREFIX}"
  run_remote_script 'set -eu
release_prefix="$1"
retention="$2"

case "$retention" in
  *[!0-9]*|"")
    echo "DEPLOY_RETENTION non numerico: $retention" >&2
    exit 1
    ;;
esac

count=0
docker image ls --format "{{.Repository}}:{{.Tag}}" |
  awk -v prefix="$release_prefix" "index(\$0, prefix) == 1 { print }" |
  sort -r |
  while IFS= read -r tag; do
    [ -n "$tag" ] || continue
    count=$((count + 1))
    if [ "$count" -gt "$retention" ]; then
      docker image rm "$tag" >/dev/null 2>&1 || true
    fi
  done' \
    "${IMAGE_RELEASE_TAG_PREFIX}" \
    "${DEPLOY_RETENTION}"
}

rollback_remote() {
  require_command ssh
  require_deploy_config

  log "Rollback release ${RELEASE_TS}: ${RELEASE_IMAGE_TAG} -> ${IMAGE_NAME}"
  run_remote_script 'set -eu
release_tag="$1"
image_name="$2"
archive_tag="$3"

docker image inspect "$release_tag" >/dev/null || {
  echo "release image tag non trovata: $release_tag" >&2
  exit 1
}

docker tag "$release_tag" "$image_name"
docker tag "$release_tag" "$archive_tag"' \
    "${RELEASE_IMAGE_TAG}" \
    "${IMAGE_NAME}" \
    "${IMAGE_ARCHIVE_TAG}"
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
      [[ "${should_build}" == "1" ]] && log "Ignoro --build: deploy esegue sempre build remoto"
      require_clean_source
      resolve_source_revision
      check_remote_tools
      build_remote_image_from_stream
      restart_remote_service
      prune_remote_release_tags
      ;;
    rollback)
      [[ -n "${RELEASE_TS:-}" ]] || fail "Imposta RELEASE_TS=YYYYmmddHHMMSS per il rollback"
      set_release_vars
      rollback_remote
      restart_remote_service
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
