#!/usr/bin/env bash
# Copy dist/ipk and/or dist/apk to router and run install-on-router.sh in local mode.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROUTER="${1:-192.168.42.1}"
USER="${ROUTER_USER:-root}"
DIST="${ROOT_DIR}/dist"
REMOTE_DIR="/tmp/hf-dist"

have_ipk=false
have_apk=false
[[ -d "$DIST/ipk" && -n "$(ls -A "$DIST/ipk" 2>/dev/null)" ]] && have_ipk=true
[[ -d "$DIST/apk" && -n "$(ls -A "$DIST/apk" 2>/dev/null)" ]] && have_apk=true
$have_ipk || $have_apk || { echo "Run ./scripts/build-packages.sh first (dist/ipk or dist/apk)" >&2; exit 1; }

echo "Uploading packages to ${USER}@${ROUTER}:${REMOTE_DIR} ..."
ssh -o ConnectTimeout=15 "${USER}@${ROUTER}" "rm -rf ${REMOTE_DIR} && mkdir -p ${REMOTE_DIR}/ipk ${REMOTE_DIR}/apk"

if $have_ipk; then
	for f in "$DIST/ipk"/*.ipk; do
		[[ -f "$f" ]] || continue
		base64 <"$f" | ssh "${USER}@${ROUTER}" "base64 -d > ${REMOTE_DIR}/ipk/$(basename "$f")"
	done
fi
if $have_apk; then
	for f in "$DIST/apk"/*.apk; do
		[[ -f "$f" ]] || continue
		base64 <"$f" | ssh "${USER}@${ROUTER}" "base64 -d > ${REMOTE_DIR}/apk/$(basename "$f")"
	done
fi

base64 <"${ROOT_DIR}/scripts/install-on-router.sh" | ssh "${USER}@${ROUTER}" \
	"base64 -d > /tmp/hf-install.sh && chmod +x /tmp/hf-install.sh"

ssh "${USER}@${ROUTER}" \
	"HF_DIST_DIR=${REMOTE_DIR} HF_MODE=${HF_MODE:-full} ash /tmp/hf-install.sh"

echo "Done."
