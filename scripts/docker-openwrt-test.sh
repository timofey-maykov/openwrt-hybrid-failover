#!/usr/bin/env bash
# OpenWrt 25.12 lab: Docker (Linux + KVM) или QEMU на хосте (macOS).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
USE_QEMU="${HF_USE_QEMU:-auto}"

log() { printf '[openwrt-lab] %s\n' "$*"; }

if [[ "$USE_QEMU" = "auto" ]]; then
	case "$(uname -s)" in
		Darwin) USE_QEMU=1 ;;
		Linux)
			if [[ -e /dev/kvm ]] && [[ -r /dev/kvm ]]; then
				USE_QEMU=0
			else
				USE_QEMU=1
			fi
			;;
		*) USE_QEMU=1 ;;
	esac
fi

if [[ "$USE_QEMU" = "1" ]]; then
	log "macOS / нет KVM: образ albrechtloh/openwrt-docker не подходит (нужен /dev/kvm)."
	log "Запускаем OpenWrt 25.12 через QEMU на хосте (TCG)..."
	exec "${ROOT_DIR}/scripts/openwrt-qemu-lab.sh" "$@"
fi

# Linux + KVM: albrechtloh/openwrt-docker
CONTAINER="${HF_DOCKER_NAME:-hybrid-failover-openwrt}"
IMAGE="${HF_DOCKER_IMAGE:-albrechtloh/openwrt-docker:latest}"
LUCI_PORT="${HF_LUCI_PORT:-18080}"
SSH_PORT="${HF_SSH_PORT:-18022}"
GUI_PORT="${HF_GUI_PORT:-18006}"
PACK_DIR="${ROOT_DIR}/dist/apk"

die() { printf '[openwrt-lab] ERROR: %s\n' "$*" >&2; exit 1; }

[[ -d "$PACK_DIR" ]] || die "Сначала: ./scripts/build-packages.sh"
command -v docker >/dev/null || die "docker не найден"

docker rm -f "$CONTAINER" >/dev/null 2>&1 || true

log "Docker + KVM: ${IMAGE}"

docker run -d --name "$CONTAINER" \
	--privileged \
	--device /dev/kvm \
	-p "${GUI_PORT}:8006" \
	-p "${LUCI_PORT}:8000" \
	-p "${SSH_PORT}:8022" \
	-e "RAM_COUNT=1024" \
	-e "CPU_COUNT=2" \
	-v "${PACK_DIR}:/packages:ro" \
	"$IMAGE" >/dev/null

# shellcheck source=scripts/openwrt-qemu-lab.sh
# reuse wait logic via sourcing only functions - simpler duplicate minimal wait
for i in $(seq 10 10 600); do
	curl -fsS -o /dev/null -m 5 "http://127.0.0.1:${LUCI_PORT}/" 2>/dev/null && break
	docker ps -q -f "name=^${CONTAINER}$" | grep -q . || {
		docker logs "$CONTAINER" 2>&1 | tail -25
		die "контейнер упал"
	}
	sleep 10
	log "ожидание LuCI... (${i}s)"
done

curl -fsS -o /dev/null "http://127.0.0.1:${LUCI_PORT}/" || die "LuCI недоступен"

cat <<EOF

================================================================================
OpenWrt 25.12.4 (Docker + KVM, ${CONTAINER})

  LuCI:       http://127.0.0.1:${LUCI_PORT}/
  Dashboard:  http://127.0.0.1:${GUI_PORT}/
  SSH:        ssh -p ${SSH_PORT} root@127.0.0.1

Остановка: docker rm -f ${CONTAINER}
================================================================================

EOF
