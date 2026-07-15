#!/usr/bin/env bash
# Build and package the frontend and an embedded-frontend backend release.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${SCRIPT_DIR}/backend"
VERSION="$(<"${BACKEND_DIR}/cmd/server/VERSION")"
APP_NAME="sub2api"
BACKEND_BUILD_DIR="${SCRIPT_DIR}/build/backend"
BACKEND_PACKAGE_DIR="${BACKEND_BUILD_DIR}/package"
BACKEND_ZIP="${BACKEND_BUILD_DIR}/${APP_NAME}-backend-${VERSION}.zip"

for command in go pnpm zip; do
  if ! command -v "${command}" >/dev/null; then
    printf 'Required command not found: %s\n' "${command}" >&2
    exit 1
  fi
done

echo "==> Building and packaging frontend (${VERSION})"
"${SCRIPT_DIR}/build_frontend.sh"

echo "==> Building backend with embedded frontend (${VERSION})"
(
  cd "${BACKEND_DIR}"
  CGO_ENABLED=0 go build \
    -tags embed \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o bin/server \
    ./cmd/server
)

echo "==> Packaging backend"
rm -rf "${BACKEND_PACKAGE_DIR}"
mkdir -p "${BACKEND_PACKAGE_DIR}"
cp "${BACKEND_DIR}/bin/server" "${BACKEND_PACKAGE_DIR}/${APP_NAME}"
cp "${BACKEND_DIR}/config.yaml" "${BACKEND_PACKAGE_DIR}/config.yaml"
cp -R "${BACKEND_DIR}/resources" "${BACKEND_PACKAGE_DIR}/resources"

rm -f "${BACKEND_ZIP}"
(
  cd "${BACKEND_PACKAGE_DIR}"
  zip -qr "${BACKEND_ZIP}" .
)

printf '\nBuild complete:\n'
printf '  Frontend: %s\n' "${SCRIPT_DIR}/build/frontend/${APP_NAME}-frontend-${VERSION}.zip"
printf '  Backend:  %s\n' "${BACKEND_ZIP}"
printf '  Binary:   %s\n' "${BACKEND_DIR}/bin/server"
