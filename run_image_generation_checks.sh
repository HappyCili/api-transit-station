#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
BACKEND_DIR="$ROOT_DIR/backend"

FRONTEND_ONLY=0
BACKEND_ONLY=0
SKIP_INSTALL=0
SKIP_BUILD=0

usage() {
  cat <<'USAGE'
Usage: bash run_image_generation_checks.sh [options]

Options:
  --frontend-only   Only run frontend dependency/check/build steps.
  --backend-only    Only run backend format/test steps.
  --skip-install    Skip frontend dependency installation.
  --skip-build      Skip frontend production build.
  -h, --help        Show this help.

Environment:
  PNPM_CMD          Override pnpm command, e.g. "corepack pnpm".
  STRICT_FROZEN_LOCKFILE=1
                    Fail instead of retrying pnpm install without --frozen-lockfile.
  RUN_BACKEND=0     Skip backend checks.
  RUN_FRONTEND=0    Skip frontend checks.
USAGE
}

log() {
  printf '\n\033[1;36m==> %s\033[0m\n' "$*"
}

warn() {
  printf '\n\033[1;33mWARN: %s\033[0m\n' "$*" >&2
}

fail() {
  printf '\n\033[1;31mERROR: %s\033[0m\n' "$*" >&2
  exit 1
}

run() {
  printf '+ %s\n' "$*"
  "$@"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --frontend-only)
      FRONTEND_ONLY=1
      ;;
    --backend-only)
      BACKEND_ONLY=1
      ;;
    --skip-install)
      SKIP_INSTALL=1
      ;;
    --skip-build)
      SKIP_BUILD=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "Unknown option: $1"
      ;;
  esac
  shift
done

if [[ "$FRONTEND_ONLY" -eq 1 && "$BACKEND_ONLY" -eq 1 ]]; then
  fail "--frontend-only and --backend-only cannot be used together"
fi

should_run_frontend() {
  [[ "${RUN_FRONTEND:-1}" != "0" && "$BACKEND_ONLY" -eq 0 ]]
}

should_run_backend() {
  [[ "${RUN_BACKEND:-1}" != "0" && "$FRONTEND_ONLY" -eq 0 ]]
}

resolve_pnpm() {
  if [[ -n "${PNPM_CMD:-}" ]]; then
    printf '%s\n' "$PNPM_CMD"
    return 0
  fi

  if command -v pnpm >/dev/null 2>&1; then
    printf '%s\n' "pnpm"
    return 0
  fi

  if command -v corepack >/dev/null 2>&1; then
    log "pnpm not found; enabling pnpm through corepack" >&2
    run corepack enable >&2
    run corepack prepare pnpm@latest --activate >&2
    printf '%s\n' "pnpm"
    return 0
  fi

  return 1
}

run_pnpm() {
  local pnpm_cmd="$1"
  shift
  read -r -a pnpm_parts <<< "$pnpm_cmd"
  CI=true run "${pnpm_parts[@]}" "$@"
}

install_frontend_dependencies() {
  local pnpm_cmd="$1"

  if [[ "$SKIP_INSTALL" -eq 1 ]]; then
    warn "Skipping frontend dependency installation"
    return 0
  fi

  log "Installing frontend dependencies from pnpm-lock.yaml"
  if run_pnpm "$pnpm_cmd" install --frozen-lockfile; then
    return 0
  fi

  if [[ "${STRICT_FROZEN_LOCKFILE:-0}" == "1" ]]; then
    fail "pnpm install --frozen-lockfile failed and STRICT_FROZEN_LOCKFILE=1 is set"
  fi

  warn "Frozen pnpm install failed; retrying with plain pnpm install to refresh local lockfile/install state"
  run_pnpm "$pnpm_cmd" install
}

frontend_checks() {
  [[ -d "$FRONTEND_DIR" ]] || fail "frontend directory not found: $FRONTEND_DIR"

  local pnpm_cmd
  pnpm_cmd="$(resolve_pnpm)" || fail "pnpm is not available. Install pnpm or enable corepack first."

  cd "$FRONTEND_DIR"

  install_frontend_dependencies "$pnpm_cmd"

  log "Running frontend lint check"
  run_pnpm "$pnpm_cmd" run lint:check

  log "Running frontend type check"
  run_pnpm "$pnpm_cmd" run typecheck

  if [[ "$SKIP_BUILD" -eq 0 ]]; then
    log "Running frontend production build"
    run_pnpm "$pnpm_cmd" run build
  else
    warn "Skipping frontend production build"
  fi
}

backend_checks() {
  [[ -d "$BACKEND_DIR" ]] || fail "backend directory not found: $BACKEND_DIR"

  if ! command -v go >/dev/null 2>&1; then
    warn "go is not available; backend gofmt/go test steps skipped"
    return 0
  fi

  cd "$BACKEND_DIR"

  local go_files=(
    "internal/service/image_generation.go"
    "internal/repository/image_generation_repo.go"
    "internal/handler/image_generation_handler.go"
    "internal/handler/openai_uploads.go"
    "internal/service/openai_images.go"
    "internal/server/routes/gateway.go"
    "internal/server/routes/user.go"
    "internal/handler/handler.go"
    "internal/handler/wire.go"
    "internal/repository/wire.go"
    "internal/service/wire.go"
    "cmd/server/wire_gen.go"
  )

  log "Formatting touched backend Go files"
  run gofmt -w "${go_files[@]}"

  log "Running backend tests"
  run go test ./...
}

main() {
  log "Starting image generation feature checks"

  if should_run_frontend; then
    frontend_checks
  else
    warn "Frontend checks skipped"
  fi

  if should_run_backend; then
    backend_checks
  else
    warn "Backend checks skipped"
  fi

  log "All requested checks finished"
}

main "$@"
