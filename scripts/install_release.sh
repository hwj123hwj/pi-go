#!/usr/bin/env bash
set -euo pipefail

: "${DEPLOY_PATH:?DEPLOY_PATH is required}"
: "${RELEASE_NAME:?RELEASE_NAME is required}"
: "${SERVICE_NAME:?SERVICE_NAME is required}"
: "${BINARY_NAME:?BINARY_NAME is required}"

ARCHIVE="/tmp/${RELEASE_NAME}.tar.gz"
SERVICE_FILE="/tmp/${SERVICE_NAME}.service"
RELEASES_DIR="${DEPLOY_PATH}/releases"
SHARED_DIR="${DEPLOY_PATH}/shared"
CURRENT_LINK="${DEPLOY_PATH}/current"
TARGET_RELEASE_DIR="${RELEASES_DIR}/${RELEASE_NAME}"

mkdir -p "${RELEASES_DIR}" "${SHARED_DIR}"

if [[ ! -f "${ARCHIVE}" ]]; then
  echo "release archive not found: ${ARCHIVE}" >&2
  exit 1
fi

rm -rf "${TARGET_RELEASE_DIR}"
tar -C "${RELEASES_DIR}" -xzf "${ARCHIVE}"

if [[ ! -f "${TARGET_RELEASE_DIR}/${BINARY_NAME}" ]]; then
  echo "binary not found after extract: ${TARGET_RELEASE_DIR}/${BINARY_NAME}" >&2
  exit 1
fi

chmod +x "${TARGET_RELEASE_DIR}/${BINARY_NAME}"
ln -sfn "${TARGET_RELEASE_DIR}" "${CURRENT_LINK}"

if [[ -f "${SERVICE_FILE}" ]]; then
  cp "${SERVICE_FILE}" "/etc/systemd/system/${SERVICE_NAME}.service"
fi

if [[ ! -f "${SHARED_DIR}/.env" ]]; then
  cat > "${SHARED_DIR}/.env" <<'EOF'
PI_GO_PROVIDER=openai
PI_GO_HOST=127.0.0.1
PI_GO_PORT=8080
EOF
fi

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service"
systemctl restart "${SERVICE_NAME}.service"
systemctl --no-pager --full status "${SERVICE_NAME}.service"
