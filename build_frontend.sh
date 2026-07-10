#!/usr/bin/env bash
# Build the frontend assets and package them as a zip archive.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
APP_NAME="sub2api"
VERSION="$(tr -d '\r\n' < "${REPO_ROOT}/backend/cmd/server/VERSION")"
FRONTEND_DIST="${REPO_ROOT}/backend/internal/web/dist"
BUILD_DIR="${REPO_ROOT}/build/frontend"
ZIP_PATH="${BUILD_DIR}/${APP_NAME}-frontend-${VERSION}.zip"

command -v zip >/dev/null

echo "Building frontend..."
pnpm --dir "${REPO_ROOT}/frontend" run build

echo "Packaging frontend..."
mkdir -p "${BUILD_DIR}"
rm -f "${ZIP_PATH}"
(
  cd "${FRONTEND_DIST}"
  zip -qr "${ZIP_PATH}" .
)

echo "Frontend package created: ${ZIP_PATH}"
