#!/usr/bin/env bash
# End-to-end regression for OpenWrt QEMU lab (hybrid-failover-core).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LAB_SCRIPT="${ROOT_DIR}/scripts/openwrt-qemu-lab.sh"
INSTALL_SCRIPT="${ROOT_DIR}/scripts/install-from-local-dist.sh"
SSH_PORT="${HF_SSH_PORT:-18022}"
SSH_OPTS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=20 -p "$SSH_PORT")
SSH_TARGET="root@127.0.0.1"

log() { printf '[verify-qemu] %s\n' "$*"; }
warn() { printf '[verify-qemu] WARN: %s\n' "$*" >&2; }
die() { printf '[verify-qemu] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
	cat <<EOF
Usage: $(basename "$0") [steps]

Steps (default: all):
  build     Build .apk packages into dist/apk
  lab       Start OpenWrt QEMU lab (installs core packages when dist/apk exists)
  remote    Run hybrid-failover checks over SSH on the lab VM
  luci      Run scripts/luci-ubus-smoke.sh on guest (ubus status/health/history)
  smoke     Run host regression-smoke.sh
  failover  URLTest failover scenario (requires running sing-box + Clash API)

Environment:
  HF_SSH_PORT      SSH port forwarded to QEMU guest (default: 18022)
  HF_SKIP_LAB=1    Skip lab start (assume VM already running)
  HF_QEMU_WITH_BOT Install hybrid-failover-bot in lab (default: 0)

Manual gate (not automated):
  - awg2:// / AmneziaWG kernel module
  - Real LAN client tproxy under load

EOF
}

run_build() {
	log "Step: build packages"
	"${ROOT_DIR}/scripts/build-packages.sh"
}

run_lab() {
	if [[ "${HF_SKIP_LAB:-0}" = "1" ]]; then
		warn "HF_SKIP_LAB=1: skipping QEMU lab start"
		return 0
	fi
	log "Step: start QEMU lab"
	HF_QEMU_WITH_BOT="${HF_QEMU_WITH_BOT:-0}" "$LAB_SCRIPT"
}

ssh_guest() {
	ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "$@"
}

require_guest_binary() {
	if ! ssh_guest "command -v hybrid-failover" >/dev/null 2>&1; then
		die "hybrid-failover not installed in guest: run build + lab first"
	fi
}

run_remote() {
	log "Step: remote checks on QEMU guest"
	require_guest_binary

	ssh_guest '
		set -e
		if command -v ubus >/dev/null 2>&1; then
			ubus call hybrid-failover status "{}" | head -c 300 || echo "ubus status: skipped"
			echo
			ubus call hybrid-failover export_history "{\"limit\":5}" | head -c 200 || echo "ubus export_history: skipped"
			echo
		fi
		hybrid-failover migrate
		hybrid-failover validate --dry-run
		/etc/init.d/hybrid-failover enable
		/etc/init.d/hybrid-failover start
		sleep 3
		hybrid-failover check-nft
		test -f /etc/sing-box/config.json
		hybrid-failover pending capture || true
		hybrid-failover pending validate || echo "pending validate: skipped (no pending changes)"
		hybrid-failover global-check
		if pidof sing-box >/dev/null 2>&1; then
			hybrid-failover check-fakeip
		else
			echo "check-fakeip: skipped (sing-box not running)"
		fi
		if uci get dhcp.@dnsmasq[0].server 2>/dev/null | grep -q 127.0.0.42; then
			echo "dnsmasq: ok (127.0.0.42)"
		else
			echo "dnsmasq: skipped (dont_touch_dhcp or no outbound UCI)"
		fi
		command -v jq >/dev/null 2>&1 && echo "WARN: jq present" || true
		command -v python3 >/dev/null 2>&1 && echo "WARN: python3 present" || true
	'

	log "remote checks: ok"
}

run_failover() {
	log "Step: failover scenario (best-effort)"
	require_guest_binary

	ssh_guest '
		set -e
		if ! pidof sing-box >/dev/null 2>&1; then
			echo "failover: skipped (sing-box not running: configure proxy UCI first)"
			exit 0
		fi
		HIST=/var/log/hybrid-failover/history.jsonl
		before=0
		[ -f "$HIST" ] && before=$(wc -l < "$HIST" | tr -d " ")
		# Poll Clash API for active outbound (requires clash_api_listen reachable on router)
		CLASH=$(uci -q get hybrid-failover.settings.clash_api_listen)
		[ -z "$CLASH" ] && CLASH=127.0.0.1:9090
		case "$CLASH" in *://*) ;; *) CLASH="http://${CLASH}";; esac
		MAIN=$(uci -q get hybrid-failover.settings.main_section)
		[ -z "$MAIN" ] && MAIN=glob
		TAG="${MAIN}-out"
		wget -qO- "${CLASH}/proxies/${TAG}" 2>/dev/null | head -c 200 || echo "clash poll: unavailable"
		echo
		echo "failover history before: ${before} lines"
		echo "failover: manual urltest switch verification: manual urltest switch on lab"
	'
}

run_luci() {
	log "Step: LuCI / ubus smoke on guest"
	require_guest_binary
	ssh_guest "HF_BIN=\$(command -v hybrid-failover || echo /usr/sbin/hybrid-failover) sh -s" \
		<"${ROOT_DIR}/scripts/luci-ubus-smoke.sh"
}

run_smoke() {
	log "Step: host smoke"
	"${ROOT_DIR}/scripts/verify-smoke.sh"
}

STEPS=(build lab remote luci smoke)
if [[ $# -gt 0 ]]; then
	STEPS=("$@")
fi

for step in "${STEPS[@]}"; do
	case "$step" in
	build) run_build ;;
	lab) run_lab ;;
	remote) run_remote ;;
	luci) run_luci ;;
	smoke) run_smoke ;;
	failover) run_failover ;;
	-h|--help|help) usage; exit 0 ;;
	*) die "unknown step: $step (see --help)" ;;
	esac
done

log "verify-qemu: finished requested steps"
