#!/usr/bin/env bash
# Build frontend assets and a verified Linux x86_64 release package.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${SCRIPT_DIR}/backend"
VERSION="$(<"${BACKEND_DIR}/cmd/server/VERSION")"
COMMIT="$(git -C "${SCRIPT_DIR}" rev-parse --short HEAD)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
APP_NAME="sub2api"
BACKEND_BINARY="${BACKEND_DIR}/bin/server"
BACKEND_BUILD_DIR="${SCRIPT_DIR}/build/backend/linux-amd64"
BACKEND_PACKAGE_DIR="${BACKEND_BUILD_DIR}/package"
BACKEND_PACKAGE_BINARY="${BACKEND_PACKAGE_DIR}/${APP_NAME}"
BACKEND_ZIP="${BACKEND_BUILD_DIR}/${APP_NAME}-backend-${VERSION}-linux-amd64.zip"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

for command in go pnpm zip unzip file strings; do
  require_command "${command}"
done

echo "==> Building and packaging frontend (${VERSION})"
"${SCRIPT_DIR}/build_frontend.sh"

echo "==> Building Linux amd64 backend with embedded frontend (${VERSION})"
rm -rf "${BACKEND_PACKAGE_DIR}"
mkdir -p "${BACKEND_PACKAGE_DIR}" "$(dirname "${BACKEND_BINARY}")"
(
  cd "${BACKEND_DIR}"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -tags embed \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=release" \
    -o "${BACKEND_BINARY}" \
    ./cmd/server
)

binary_description="$(file "${BACKEND_BINARY}")"
if [[ "${binary_description}" != *"ELF 64-bit LSB executable"*"x86-64"* ]]; then
  printf 'Unexpected backend binary architecture: %s\n' "${binary_description}" >&2
  exit 1
fi

echo "==> Packaging Linux amd64 backend"
cp "${BACKEND_BINARY}" "${BACKEND_PACKAGE_BINARY}"
cp "${BACKEND_DIR}/config.yaml" "${BACKEND_PACKAGE_DIR}/config.yaml"
cp -R "${BACKEND_DIR}/resources" "${BACKEND_PACKAGE_DIR}/resources"

rm -f "${BACKEND_ZIP}"
(
  cd "${BACKEND_PACKAGE_DIR}"
  zip -qr "${BACKEND_ZIP}" .
)

echo "==> Verifying package"
unzip -t "${BACKEND_ZIP}" >/dev/null
if ! strings "${BACKEND_BINARY}" | grep -Fq "${VERSION}"; then
  printf 'Embedded version string not found: %s\n' "${VERSION}" >&2
  exit 1
fi

if command -v shasum >/dev/null 2>&1; then
  checksum="$(shasum -a 256 "${BACKEND_ZIP}" | awk '{print $1}')"
else
  checksum="$(sha256sum "${BACKEND_ZIP}" | awk '{print $1}')"
fi

printf '\nBuild complete:\n'
printf '  Frontend: %s\n' "${SCRIPT_DIR}/build/frontend/${APP_NAME}-frontend-${VERSION}.zip"
printf '  Backend:  %s\n' "${BACKEND_ZIP}"
printf '  Binary:   %s\n' "${BACKEND_BINARY}"
printf '  SHA-256:  %s\n' "${checksum}"
