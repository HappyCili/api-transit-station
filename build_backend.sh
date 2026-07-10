#!/usr/bin/env bash
# Build the backend binary and package it as a zip archive.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}"
APP_NAME="sub2api"
VERSION="$(tr -d '\r\n' < "${REPO_ROOT}/backend/cmd/server/VERSION")"
BUILD_DIR="${REPO_ROOT}/build/backend"
PACKAGE_DIR="${BUILD_DIR}/package"
ZIP_PATH="${BUILD_DIR}/${APP_NAME}-backend-${VERSION}.zip"

command -v zip >/dev/null

echo "Building backend..."
make -C "${REPO_ROOT}/backend" build

echo "Packaging backend..."
rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}"

cp "${REPO_ROOT}/backend/bin/server" "${PACKAGE_DIR}/${APP_NAME}"
cp "${REPO_ROOT}/backend/config.yaml" "${PACKAGE_DIR}/config.yaml"
cp -R "${REPO_ROOT}/backend/resources" "${PACKAGE_DIR}/resources"

rm -f "${ZIP_PATH}"
(
  cd "${PACKAGE_DIR}"
  zip -qr "${ZIP_PATH}" .
)

echo "Backend package created: ${ZIP_PATH}"
