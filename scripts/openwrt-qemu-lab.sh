#!/usr/bin/env bash
# OpenWrt 25.12 в QEMU (TCG): macOS без KVM. На Apple Silicon: native arm64 (быстро).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LAB_DIR="${ROOT_DIR}/dist/qemu-lab"
PACK_DIR="${ROOT_DIR}/dist/apk"
LUCI_PORT="${HF_LUCI_PORT:-18080}"
SSH_PORT="${HF_SSH_PORT:-18022}"
PIDFILE="${LAB_DIR}/qemu.pid"
LOGFILE="${LAB_DIR}/qemu.log"

log() { printf '[qemu-lab] %s\n' "$*"; }
warn() { printf '[qemu-lab] WARN: %s\n' "$*" >&2; }
die() { printf '[qemu-lab] ERROR: %s\n' "$*" >&2; exit 1; }

stop_qemu() {
	if [[ -f "$PIDFILE" ]]; then
		local pid
		pid="$(cat "$PIDFILE")"
		kill "$pid" 2>/dev/null || true
		rm -f "$PIDFILE"
		log "QEMU остановлен (pid ${pid})"
	fi
}

if [[ "${1:-}" = "stop" ]]; then
	stop_qemu
	exit 0
fi

HOST_ARCH="$(uname -m)"
mkdir -p "$LAB_DIR"

if [[ "$HOST_ARCH" = "arm64" || "$HOST_ARCH" = "aarch64" ]]; then
	QEMU_BIN="qemu-system-aarch64"
	IMG_NAME="openwrt-25.12.0-armsr-armv8-generic-ext4-combined-efi.img"
	IMG_GZ="${LAB_DIR}/${IMG_NAME}.gz"
	IMG="${LAB_DIR}/${IMG_NAME}"
	IMG_URL="${HF_OPENWRT_IMG_URL:-https://downloads.openwrt.org/releases/25.12.0/targets/armsr/armv8/${IMG_NAME}.gz}"
	APK_ARCH="aarch64_cortex-a53"
	QEMU_BIOS="${HF_QEMU_BIOS:-/opt/homebrew/share/qemu/edk2-aarch64-code.fd}"
	QEMU_EXTRA=(-M virt -cpu cortex-a72 -bios "$QEMU_BIOS")
else
	QEMU_BIOS=""
	QEMU_BIN="qemu-system-x86_64"
	IMG_NAME="openwrt-25.12.0-x86-64-generic-ext4-combined.img"
	IMG_GZ="${LAB_DIR}/${IMG_NAME}.gz"
	IMG="${LAB_DIR}/${IMG_NAME}"
	IMG_URL="${HF_OPENWRT_IMG_URL:-https://downloads.openwrt.org/releases/25.12.0/targets/x86/64/${IMG_NAME}.gz}"
	APK_ARCH="x86_64"
	QEMU_EXTRA=(-cpu qemu64)
fi

command -v "$QEMU_BIN" >/dev/null || die "Установите QEMU: brew install qemu"
[[ -z "$QEMU_BIOS" || -f "$QEMU_BIOS" ]] || die "EFI bios не найден: ${QEMU_BIOS} (brew install qemu)"

if [[ ! -f "$IMG" ]]; then
	log "Скачивание ${IMG_NAME} ..."
	curl -fL --progress-bar -o "$IMG_GZ" "$IMG_URL"
	log "Распаковка..."
	gunzip -kf "$IMG_GZ"
fi

if [[ -f "$PIDFILE" ]] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
	log "QEMU уже запущен (pid $(cat "$PIDFILE"))"
else
	stop_qemu
	log "Запуск ${QEMU_BIN} (TCG, arch ${HOST_ARCH})..."
	# shellcheck disable=SC2086
	"$QEMU_BIN" \
		-m 1024 -smp 2 \
		-accel tcg \
		"${QEMU_EXTRA[@]}" \
		-drive "file=${IMG},if=virtio,format=raw" \
		-netdev "user,id=lan,net=192.168.1.0/24,dhcpstart=192.168.1.100,hostfwd=tcp::${LUCI_PORT}-192.168.1.1:80,hostfwd=tcp::${SSH_PORT}-192.168.1.1:22" \
		-device virtio-net-pci,netdev=lan \
		-display none \
		-daemonize \
		-pidfile "$PIDFILE" \
		>>"$LOGFILE" 2>&1
	sleep 2
	kill -0 "$(cat "$PIDFILE")" 2>/dev/null || {
		cat "$LOGFILE" 2>/dev/null || true
		die "QEMU не стартовал"
	}
fi

wait_http() {
	local url="$1" max="${2:-600}" i=0
	while [[ "$i" -lt "$max" ]]; do
		if curl -fsS -o /dev/null -m 5 "$url" 2>/dev/null; then
			return 0
		fi
		if [[ -f "$PIDFILE" ]] && ! kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
			cat "$LOGFILE" 2>/dev/null || true
			return 1
		fi
		i=$((i + 10))
		sleep 10
		log "ожидание LuCI... (${i}s / ${max}s)"
	done
	return 1
}

if ! wait_http "http://127.0.0.1:${LUCI_PORT}/" 600; then
	die "LuCI не ответил: лог: ${LOGFILE}"
fi

log "LuCI доступен."

ssh_upload() {
	local src="$1" dest="$2"
	cat "$src" | ssh $SSH_OPTS root@127.0.0.1 "cat > $dest"
}

if [[ -d "$PACK_DIR" ]]; then
	SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=20 -p ${SSH_PORT}"
	core="$(ls "${PACK_DIR}/hybrid-failover-core-"*"_${APK_ARCH}.apk" 2>/dev/null | head -1)"
	luci="$(ls "${PACK_DIR}/luci-app-hybrid-failover_"*.apk 2>/dev/null | head -1)"
	bot="$(ls "${PACK_DIR}/hybrid-failover-bot-"*"_${APK_ARCH}.apk" 2>/dev/null | head -1)"
	if [[ -n "$core" ]] && ssh $SSH_OPTS root@127.0.0.1 "command -v apk" 2>/dev/null | grep -q apk; then
		log "Установка hybrid-failover-core + LuCI (${APK_ARCH})..."
		ssh_upload "$core" "/tmp/$(basename "$core")" || warn "upload core failed"
		[[ -n "$luci" ]] && ssh_upload "$luci" "/tmp/$(basename "$luci")" || warn "upload luci failed"
		if [[ "${HF_QEMU_WITH_BOT:-0}" = "1" && -n "$bot" ]]; then
			ssh_upload "$bot" "/tmp/$(basename "$bot")" || warn "upload bot failed"
			for f in "${PACK_DIR}"/luci-app-hybrid-failover-bot-*.apk; do
				[[ -f "$f" ]] && ssh_upload "$f" "/tmp/$(basename "$f")" || true
			done
		fi
		ssh $SSH_OPTS root@127.0.0.1 '
			apk update 2>/dev/null || true
			for f in /tmp/hybrid-failover-core-*.apk /tmp/luci-app-hybrid-failover_*.apk /tmp/hybrid-failover-bot-*.apk /tmp/luci-app-hybrid-failover-bot-*.apk; do
				[ -f "$f" ] && apk add --allow-untrusted "$f" 2>/dev/null || true
			done
			hybrid-failover migrate 2>/dev/null || true
			/etc/init.d/uhttpd restart 2>/dev/null || true
			rm -rf /tmp/luci-modulecache/* /tmp/luci-indexcache/* 2>/dev/null || true
		' 2>/dev/null || warn "apk install incomplete"
	else
		warn "hybrid-failover-core .apk для ${APK_ARCH} не найден или SSH не готов"
	fi
fi

cat <<EOF

================================================================================
OpenWrt 25.12.0 (${QEMU_BIN}, pid $(cat "$PIDFILE"))

  LuCI:              http://127.0.0.1:${LUCI_PORT}/
  HF LuCI: http://127.0.0.1:${LUCI_PORT}/cgi-bin/luci/admin/services/hybrid-failover
  SSH:               ssh -p ${SSH_PORT} root@127.0.0.1

Остановка:  ${0} stop
Лог:        ${LOGFILE}
================================================================================

EOF
