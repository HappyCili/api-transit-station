#!/usr/bin/env bash
# Re-run with bash when invoked as `sh run_image_generation_dev.sh`.
if [ -z "${BASH_VERSION:-}" ]; then
  exec /usr/bin/env bash "$0" "$@"
fi
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
BACKEND_DIR="$ROOT_DIR/backend"

HOST="${HOST:-127.0.0.1}"
FRONTEND_PORT="${FRONTEND_PORT:-59527}"
PROXY_TARGET="${VITE_DEV_PROXY_TARGET:-http://localhost:59528}"
BACKEND_CONFIG="${BACKEND_CONFIG:-}"
FRONTEND_ONLY=0
BACKEND_ONLY=0
SKIP_INSTALL=0

BACKEND_PID=""
FRONTEND_PID=""
BACKEND_PARTS=()
BACKEND_CONFIG_TMP_DIR=""
BACKEND_CONFIG_EFFECTIVE=""

usage() {
  cat <<'USAGE'
Usage: ./run_image_generation_dev.sh [options]

Start the Sub2API backend and frontend dev server for the image generation page.
Existing listeners on the selected frontend port and configured backend port are
stopped before their replacements are started.

Options:
  --frontend-only          Only start the frontend dev server.
  --backend-only           Only start the backend server.
  --skip-install           Skip frontend pnpm install.
  --host <host>            Frontend dev server host. Default: 127.0.0.1
  --frontend-port <port>   Frontend dev server port. Default: 59527
  --proxy-target <url>     Frontend proxy target. Default: http://localhost:59528
  --backend-config <path>  Backend config yaml. Default: backend/config.local.yaml if present,
                           otherwise backend/config.yaml.
  -h, --help              Show this help.

Environment:
  PNPM_CMD                 Override pnpm command, e.g. "corepack pnpm".
  STRICT_FROZEN_LOCKFILE=1 Fail instead of retrying pnpm install without --frozen-lockfile.
  BACKEND_CMD              Override backend command, e.g. "go run ./cmd/server".
  BACKEND_CONFIG           Same as --backend-config.
  FRONTEND_PORT            Same as --frontend-port.
  HOST                     Same as --host.
  VITE_DEV_PROXY_TARGET    Same as --proxy-target.

Examples:
  ./run_image_generation_dev.sh
  ./run_image_generation_dev.sh --skip-install
  ./run_image_generation_dev.sh --frontend-only --proxy-target http://localhost:59528
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

validate_port() {
  local port="$1"
  local label="$2"

  [[ "$port" =~ ^[0-9]+$ ]] || fail "$label port must be a number: $port"
  (( port >= 1 && port <= 65535 )) || fail "$label port must be between 1 and 65535: $port"
}

listener_pids() {
  local port="$1"
  lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
}

stop_port_listeners() {
  local port="$1"
  local label="$2"
  local pids=""
  local pid=""
  local attempt=""

  validate_port "$port" "$label"
  command -v lsof >/dev/null 2>&1 || fail "lsof is required to replace an existing $label listener"

  pids="$(listener_pids "$port")"
  [[ -n "$pids" ]] || return 0

  log "Stopping existing $label listener(s) on port $port: $pids"
  for pid in $pids; do
    kill "$pid" >/dev/null 2>&1 || true
  done

  for attempt in {1..20}; do
    sleep 0.25
    pids="$(listener_pids "$port")"
    [[ -n "$pids" ]] || return 0
  done

  warn "Force stopping existing $label listener(s) on port $port: $pids"
  for pid in $pids; do
    kill -9 "$pid" >/dev/null 2>&1 || true
  done

  for attempt in {1..20}; do
    sleep 0.25
    pids="$(listener_pids "$port")"
    [[ -n "$pids" ]] || return 0
  done

  fail "Unable to stop existing $label listener(s) on port $port: $pids"
}

backend_port_from_config() {
  local config_path="$1"
  local port=""

  port="$(awk '
    /^[[:space:]]*server:[[:space:]]*$/ { in_server = 1; next }
    in_server && /^[^[:space:]]/ { exit }
    in_server && /^[[:space:]]+port:[[:space:]]*/ {
      value = $0
      sub(/^[[:space:]]*port:[[:space:]]*/, "", value)
      sub(/[[:space:]#].*$/, "", value)
      print value
      exit
    }
  ' "$config_path")"
  [[ -n "$port" ]] || fail "Unable to read server.port from backend config: $config_path"
  validate_port "$port" "backend"
  printf '%s\n' "$port"
}

cleanup() {
  local status=$?
  trap - INT TERM EXIT

  if [[ -n "$FRONTEND_PID" ]] && kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
    warn "Stopping frontend dev server pid=$FRONTEND_PID"
    kill "$FRONTEND_PID" >/dev/null 2>&1 || true
  fi

  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
    warn "Stopping backend server pid=$BACKEND_PID"
    kill "$BACKEND_PID" >/dev/null 2>&1 || true
  fi

  if [[ -n "$FRONTEND_PID" ]]; then
    wait "$FRONTEND_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$BACKEND_PID" ]]; then
    wait "$BACKEND_PID" >/dev/null 2>&1 || true
  fi

  if [[ -n "$BACKEND_CONFIG_TMP_DIR" && -d "$BACKEND_CONFIG_TMP_DIR" ]]; then
    rm -rf "$BACKEND_CONFIG_TMP_DIR"
  fi
  exit "$status"
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
    --host)
      [[ $# -ge 2 ]] || fail "--host requires a value"
      HOST="$2"
      shift
      ;;
    --frontend-port)
      [[ $# -ge 2 ]] || fail "--frontend-port requires a value"
      FRONTEND_PORT="$2"
      shift
      ;;
    --proxy-target)
      [[ $# -ge 2 ]] || fail "--proxy-target requires a value"
      PROXY_TARGET="$2"
      shift
      ;;
    --backend-config)
      [[ $# -ge 2 ]] || fail "--backend-config requires a value"
      BACKEND_CONFIG="$2"
      shift
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

install_frontend_dependencies() {
  local pnpm_cmd="$1"
  read -r -a pnpm_parts <<< "$pnpm_cmd"

  if [[ "$SKIP_INSTALL" -eq 1 ]]; then
    warn "Skipping frontend dependency installation"
    return 0
  fi

  log "Installing frontend dependencies"
  cd "$FRONTEND_DIR"
  if CI=true run "${pnpm_parts[@]}" install --frozen-lockfile; then
    return 0
  fi

  if [[ "${STRICT_FROZEN_LOCKFILE:-0}" == "1" ]]; then
    fail "pnpm install --frozen-lockfile failed and STRICT_FROZEN_LOCKFILE=1 is set"
  fi

  warn "Frozen pnpm install failed; retrying with plain pnpm install to refresh local lockfile/install state"
  CI=true run "${pnpm_parts[@]}" install
}

start_backend() {
  [[ -d "$BACKEND_DIR" ]] || fail "backend directory not found: $BACKEND_DIR"

  cd "$BACKEND_DIR"

  prepare_backend_config
  stop_port_listeners "$(backend_port_from_config "$BACKEND_CONFIG_EFFECTIVE")" "backend"
  resolve_backend_runner

  log "Starting backend: ${BACKEND_PARTS[*]}"
  if [[ -n "$BACKEND_CONFIG_TMP_DIR" ]]; then
    printf 'Backend config: %s\n' "$BACKEND_CONFIG_EFFECTIVE"
    DATA_DIR="$BACKEND_CONFIG_TMP_DIR" "${BACKEND_PARTS[@]}" &
  else
    printf 'Backend config: %s\n' "$BACKEND_CONFIG_EFFECTIVE"
    "${BACKEND_PARTS[@]}" &
  fi
  BACKEND_PID=$!
  printf 'Backend pid: %s\n' "$BACKEND_PID"
}

prepare_backend_config() {
  local selected_config=""

  if [[ -n "$BACKEND_CONFIG" ]]; then
    selected_config="$BACKEND_CONFIG"
  elif [[ -f "$BACKEND_DIR/config.local.yaml" ]]; then
    selected_config="$BACKEND_DIR/config.local.yaml"
  elif [[ -f "$BACKEND_DIR/config.yaml" ]]; then
    selected_config="$BACKEND_DIR/config.yaml"
  fi

  [[ -n "$selected_config" ]] || fail "No backend config file found"
  [[ -f "$selected_config" ]] || fail "Backend config file not found: $selected_config"

  selected_config="$(cd "$(dirname "$selected_config")" && pwd)/$(basename "$selected_config")"
  BACKEND_CONFIG_EFFECTIVE="$selected_config"

  if [[ "$(basename "$selected_config")" == "config.yaml" ]]; then
    if [[ "$(dirname "$selected_config")" == "$BACKEND_DIR" ]]; then
      BACKEND_CONFIG_TMP_DIR=""
      return 0
    fi
  fi

  BACKEND_CONFIG_TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-dev-config.XXXXXX")"
  cp "$selected_config" "$BACKEND_CONFIG_TMP_DIR/config.yaml"
}

resolve_backend_runner() {
  if [[ -n "${BACKEND_CMD:-}" ]]; then
    read -r -a BACKEND_PARTS <<< "$BACKEND_CMD"
    return 0
  fi

  if command -v go >/dev/null 2>&1; then
    BACKEND_PARTS=(go run ./cmd/server)
    return 0
  fi

  if binary_matches_host "./server"; then
    BACKEND_PARTS=(./server)
    return 0
  fi

  if binary_matches_host "./sub2api"; then
    BACKEND_PARTS=(./sub2api)
    return 0
  fi

  warn "No usable backend runner found for $(uname -s)/$(uname -m)."
  warn "Existing backend binaries may be built for another platform:"
  if [[ -e "./server" ]]; then
    file "./server" >&2 || true
  fi
  if [[ -e "./sub2api" ]]; then
    file "./sub2api" >&2 || true
  fi
  fail "Install Go, build a native backend binary, set BACKEND_CMD, or run with --frontend-only."
}

binary_matches_host() {
  local path="$1"
  [[ -x "$path" ]] || return 1

  local info os arch
  info="$(file "$path" 2>/dev/null || true)"
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os:$arch" in
    Darwin:arm64)
      [[ "$info" == *"Mach-O 64-bit executable arm64"* || "$info" == *"Mach-O 64-bit executable universal binary"* ]]
      ;;
    Darwin:x86_64)
      [[ "$info" == *"Mach-O 64-bit executable x86_64"* || "$info" == *"Mach-O 64-bit executable universal binary"* ]]
      ;;
    Linux:x86_64)
      [[ "$info" == *"ELF 64-bit"* && "$info" == *"x86-64"* ]]
      ;;
    Linux:aarch64|Linux:arm64)
      [[ "$info" == *"ELF 64-bit"* && ( "$info" == *"ARM aarch64"* || "$info" == *"ARM64"* ) ]]
      ;;
    *)
      return 1
      ;;
  esac
}

start_frontend() {
  [[ -d "$FRONTEND_DIR" ]] || fail "frontend directory not found: $FRONTEND_DIR"

  local pnpm_cmd="$1"
  read -r -a pnpm_parts <<< "$pnpm_cmd"

  stop_port_listeners "$FRONTEND_PORT" "frontend"
  cd "$FRONTEND_DIR"

  log "Starting frontend dev server"
  printf 'Frontend URL: http://%s:%s\n' "$HOST" "$FRONTEND_PORT"
  printf 'API proxy target: %s\n' "$PROXY_TARGET"
  VITE_DEV_PROXY_TARGET="$PROXY_TARGET" VITE_DEV_PORT="$FRONTEND_PORT" \
    "${pnpm_parts[@]}" run dev -- --host "$HOST" --port "$FRONTEND_PORT" &
  FRONTEND_PID=$!
  printf 'Frontend pid: %s\n' "$FRONTEND_PID"
}

wait_for_processes() {
  while true; do
    sleep 1

    if [[ -n "$BACKEND_PID" ]] && ! kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
      wait "$BACKEND_PID"
      return $?
    fi

    if [[ -n "$FRONTEND_PID" ]] && ! kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
      wait "$FRONTEND_PID"
      return $?
    fi
  done
}

main() {
  trap cleanup INT TERM EXIT

  log "Starting Sub2API image generation dev environment"

  local pnpm_cmd=""
  if [[ "$BACKEND_ONLY" -eq 0 ]]; then
    pnpm_cmd="$(resolve_pnpm)" || fail "pnpm is not available. Install pnpm or enable corepack first."
    install_frontend_dependencies "$pnpm_cmd"
  fi

  if [[ "$FRONTEND_ONLY" -eq 0 ]]; then
    start_backend
  else
    warn "Backend server skipped"
  fi

  if [[ "$BACKEND_ONLY" -eq 0 ]]; then
    start_frontend "$pnpm_cmd"
  else
    warn "Frontend dev server skipped"
  fi

  log "Services are running. Press Ctrl+C to stop."
  wait_for_processes
}

main "$@"
