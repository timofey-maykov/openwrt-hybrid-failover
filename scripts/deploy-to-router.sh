#!/usr/bin/env bash
# Deploy Hybrid Failover packages to an OpenWrt router over SSH.
#
# Usage:
#   ./scripts/deploy-to-router.sh [router_ip] [ssh_user]
#   HF_DIST_DIR=./dist HF_BUILD=1 ./scripts/deploy-to-router.sh 192.168.42.1
#
# HF_BUILD=1       rebuild ipk before deploy (default: use dist/ipk)
# HF_UPX           auto|1|0 for rebuild (default: auto, сжимает core/bot через UPX)
# HF_COMPONENTS    core,luci,bot (default: core,luci)
# HF_SKIP_VERIFY   skip post-install checks
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/arch-map.sh
source "$ROOT_DIR/scripts/lib/arch-map.sh"

ROUTER="${1:-192.168.42.1}"
ROUTER_USER="${2:-root}"
HF_DIST_DIR="${HF_DIST_DIR:-$ROOT_DIR/dist}"
HF_BUILD="${HF_BUILD:-0}"
HF_UPX="${HF_UPX:-auto}"
HF_COMPONENTS="${HF_COMPONENTS:-core,luci}"
HF_SKIP_VERIFY="${HF_SKIP_VERIFY:-0}"
VERSION="$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")"
PKG_RELEASE="${PKG_RELEASE:-1}"

log() { printf '[deploy] %s\n' "$*"; }
warn() { printf '[deploy] WARN: %s\n' "$*" >&2; }
die() { printf '[deploy] ERROR: %s\n' "$*" >&2; exit 1; }

ssh_cmd() {
	ssh -o BatchMode=yes -o ConnectTimeout=10 "${ROUTER_USER}@${ROUTER}" "$@"
}

scp_files() {
	scp -O "$@" "${ROUTER_USER}@${ROUTER}:/tmp/"
}

detect_remote_arch() {
	local raw
	raw="$(ssh_cmd '. /etc/openwrt_release 2>/dev/null; echo "${DISTRIB_ARCH:-unknown}"' 2>/dev/null || echo unknown)"
	normalize_openwrt_arch "$raw"
}

arch_pkg_candidates() {
	local primary="$1"
	echo "$primary"
	case "$primary" in
		aarch64_cortex-a53) echo "aarch64_generic" ;;
		aarch64_generic) echo "aarch64_cortex-a53" ;;
	esac
}

find_ipk() {
	local pkg="$1"
	local arch="$2"
	local dir="$HF_DIST_DIR/ipk"
	local ver="${VERSION}-${PKG_RELEASE}"
	local f="${dir}/${pkg}_${ver}_${arch}.ipk"
	if [ -f "$f" ]; then
		echo "$f"
		return 0
	fi
	return 1
}

resolve_ipk() {
	local pkg="$1"
	local primary="$2"
	local alt
	if [ "$pkg" = "luci-app-hybrid-failover" ] || [ "$pkg" = "luci-i18n-hybrid-failover" ] || [ "$pkg" = "luci-app-hybrid-failover-bot" ]; then
		if path="$(find_ipk "$pkg" "all" 2>/dev/null)"; then
			echo "$path"
			return 0
		fi
	fi
	while IFS= read -r alt; do
		[ -n "$alt" ] || continue
		if path="$(find_ipk "$pkg" "$alt" 2>/dev/null)"; then
			echo "$path"
			return 0
		fi
	done <<EOF
$(arch_pkg_candidates "$primary")
EOF
	return 1
}

REMOTE_ARCH="$(detect_remote_arch)"

if [ "$HF_BUILD" = "1" ]; then
	log "Building packages v${VERSION} for ${REMOTE_ARCH} (HF_UPX=${HF_UPX})..."
	(
		cd "$ROOT_DIR"
		HF_PKG_FORMAT=ipk HF_UPX="$HF_UPX" HF_BUILD_ARCHS_OVERRIDE="$REMOTE_ARCH" ./scripts/build-packages.sh
	)
fi

log "Router ${ROUTER} arch: ${REMOTE_ARCH}"

TMP_FILES=()
cleanup() {
	[ ${#TMP_FILES[@]} -eq 0 ] || ssh_cmd "rm -f ${TMP_FILES[*]}" 2>/dev/null || true
}
trap cleanup EXIT

upload_ipk() {
	local pkg="$1"
	local path
	path="$(resolve_ipk "$pkg" "$REMOTE_ARCH")" || die "No ${pkg} ipk for ${REMOTE_ARCH} in ${HF_DIST_DIR}/ipk (run ./scripts/build-packages.sh)"
	local base
	base="$(basename "$path")"
	log "Upload ${base}" >&2
	scp_files "$path"
	TMP_FILES+=("/tmp/${base}")
	echo "/tmp/${base}"
}

install_ipk() {
	local remote_path="$1"
	local pkg="$2"
	log "Install $(basename "$remote_path")" >&2
	if ! ssh_cmd "opkg install --force-reinstall --force-space --force-overwrite '${remote_path}'"; then
		die "opkg install failed for ${remote_path}"
	fi
	if ! ssh_cmd "opkg list-installed 2>/dev/null | grep -q '^${pkg} '"; then
		die "${pkg} not installed after opkg"
	fi
}

verify_router() {
	local attempt out
	for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
		if ! ssh_cmd "test -x /usr/sbin/hybrid-failover && test -x /etc/init.d/hybrid-failover"; then
			sleep 5
			continue
		fi
		if ! ssh_cmd "/usr/sbin/hybrid-failover validate" >/dev/null 2>&1; then
			sleep 5
			continue
		fi
		out="$(ssh_cmd "/usr/sbin/hybrid-failover rpc status 2>/dev/null" || true)"
		if printf '%s' "$out" | grep -q '"engine_running":true'; then
			log "Verification OK (attempt ${attempt})"
			return 0
		fi
		if ssh_cmd "test -f /var/run/hybrid-failover/engine.running"; then
			log "Verification OK (engine.running marker, attempt ${attempt})"
			return 0
		fi
		if [ "$attempt" -lt 12 ]; then
			log "Waiting for native engine (attempt ${attempt}/12)..." >&2
			sleep 5
		fi
	done
	die "post-install verification failed on router (engine not running after 60s)"
}

# Install core first so a failed LuCI install never leaves the router without hybrid-failover.
if [[ "$HF_COMPONENTS" == *core* ]]; then
	CORE_IPK="$(upload_ipk hybrid-failover-core)"
	install_ipk "$CORE_IPK" hybrid-failover-core
fi

if [[ "$HF_COMPONENTS" == *bot* ]]; then
	BOT_IPK="$(upload_ipk hybrid-failover-bot)"
	install_ipk "$BOT_IPK" hybrid-failover-bot
fi

if [[ "$HF_COMPONENTS" == *luci* ]]; then
	for pkg in luci-i18n-hybrid-failover luci-app-hybrid-failover; do
		if path="$(resolve_ipk "$pkg" "$REMOTE_ARCH")"; then
			LUCI_IPK="$(upload_ipk "$pkg")"
			install_ipk "$LUCI_IPK" "$pkg" || warn "optional package $pkg failed"
		fi
	done
fi

log "Restart hybrid-failover"
ssh_cmd "/etc/init.d/hybrid-failover restart"
ssh_cmd "/usr/sbin/hybrid-failover migrate 2>/dev/null || true"

if [ "$HF_SKIP_VERIFY" != "1" ]; then
	verify_router
fi

log "Done. LuCI: http://${ROUTER}/cgi-bin/luci/admin/services/hybrid-failover"
