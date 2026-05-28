#!/usr/bin/env bash
set -euo pipefail

ROUTER_IP="${1:-192.168.42.1}"
ROUTER_USER="${ROUTER_USER:-root}"
BIN_NAME="podkop-telegram-bot"
LOCAL_BIN="$(pwd)/bot/${BIN_NAME}"

if [[ ! -x "$LOCAL_BIN" ]]; then
  echo "Building ${BIN_NAME}..."
  (cd bot && CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags="-s -w" -o "${BIN_NAME}" ./cmd/podkop-telegram-bot)
fi

echo "Uploading binary and configs to ${ROUTER_USER}@${ROUTER_IP}..."
scp "$LOCAL_BIN" "${ROUTER_USER}@${ROUTER_IP}:/usr/bin/${BIN_NAME}"
scp "bot/openwrt/etc/init.d/${BIN_NAME}" "${ROUTER_USER}@${ROUTER_IP}:/etc/init.d/${BIN_NAME}"
scp "bot/openwrt/etc/config/${BIN_NAME}" "${ROUTER_USER}@${ROUTER_IP}:/etc/config/${BIN_NAME}"
scp "bot/openwrt/etc/${BIN_NAME}.json" "${ROUTER_USER}@${ROUTER_IP}:/etc/${BIN_NAME}.json"

ssh "${ROUTER_USER}@${ROUTER_IP}" "chmod +x /usr/bin/${BIN_NAME} /etc/init.d/${BIN_NAME} && /etc/init.d/${BIN_NAME} enable && /etc/init.d/${BIN_NAME} restart"
echo "Done."
