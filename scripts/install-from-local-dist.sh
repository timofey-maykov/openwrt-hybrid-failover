#!/usr/bin/env bash
# Copy dist/ipk to router and run install-on-router.sh in local mode.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROUTER="${1:-192.168.42.1}"
USER="${ROUTER_USER:-root}"
DIST="${ROOT_DIR}/dist"
REMOTE_DIR="/tmp/podkop-hf-dist"

[[ -d "$DIST/ipk" ]] || { echo "Run ./scripts/build-packages.sh first" >&2; exit 1; }

echo "Uploading packages to ${USER}@${ROUTER}:${REMOTE_DIR} ..."
ssh -o ConnectTimeout=15 "${USER}@${ROUTER}" "rm -rf ${REMOTE_DIR} && mkdir -p ${REMOTE_DIR}/ipk"
for ipk in "$DIST/ipk"/*.ipk; do
	base64 <"$ipk" | ssh "${USER}@${ROUTER}" "base64 -d > ${REMOTE_DIR}/ipk/$(basename "$ipk")"
done
base64 <"${ROOT_DIR}/scripts/install-on-router.sh" | ssh "${USER}@${ROUTER}" \
	"base64 -d > /tmp/podkop-install.sh && chmod +x /tmp/podkop-install.sh"

ssh "${USER}@${ROUTER}" \
	"PODKOP_HF_DIST_DIR=${REMOTE_DIR} PODKOP_HF_MODE=${PODKOP_HF_MODE:-full} ash /tmp/podkop-install.sh"

echo "Done."
