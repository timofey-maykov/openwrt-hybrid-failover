#!/usr/bin/env bash
# Build a smaller sing-box for OpenWrt routers with tight overlay (~15–25 MiB vs ~40 MiB stock).
#
# Usage:
#   ./scripts/build-sing-box-lite.sh [openwrt_arch ...]
#   SING_BOX_VERSION=1.12.22 HF_UPX=1 ./scripts/build-sing-box-lite.sh aarch64_cortex-a53
#
# Output: dist/sing-box-lite/<arch>/sing-box
#
# Install on router (overlay tight — remove old binary first):
#   /etc/init.d/hybrid-failover stop; /etc/init.d/sing-box stop
#   opkg remove --force-depends sing-box
#   cp dist/sing-box-lite/aarch64_cortex-a53/sing-box /usr/bin/sing-box
#   /etc/init.d/sing-box start; /etc/init.d/hybrid-failover start
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/arch-map.sh
source "$ROOT_DIR/scripts/lib/arch-map.sh"
# shellcheck source=scripts/lib/compress-binary.sh
source "$ROOT_DIR/scripts/lib/compress-binary.sh"

SING_BOX_VERSION="${SING_BOX_VERSION:-1.12.22}"
OUT_ROOT="${DIST_DIR:-$ROOT_DIR/dist}/sing-box-lite"
WORKDIR="${WORKDIR:-$ROOT_DIR/dist/sing-box-src}"

# Tags required by hybrid-failover (tproxy, clash API, QUIC, VLESS/Hysteria2/UTLS).
# Dropped vs stock OpenWrt sing-box: tailscale, wireguard, dhcp, acme (not used by core).
BUILD_TAGS="with_gvisor with_quic with_utls with_clash_api with_v2ray_api"

if [ "$#" -gt 0 ]; then
	ARCHS=("$@")
else
	ARCHS=(aarch64_cortex-a53 aarch64_generic arm_cortex-a7 mipsel_24kc x86_64)
fi

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "Missing: $1" >&2; exit 1; }; }
need_cmd go

if [ ! -d "$WORKDIR/.git" ]; then
	rm -rf "$WORKDIR"
	git clone --depth 1 --branch "v${SING_BOX_VERSION}" \
		https://github.com/SagerNet/sing-box.git "$WORKDIR"
fi

mkdir -p "$OUT_ROOT"

for owrt_arch in "${ARCHS[@]}"; do
	goarch="$(openwrt_arch_to_go "$owrt_arch")" || continue
	goarm="$(openwrt_arch_to_goarm "$owrt_arch")"
	gomips="$(openwrt_arch_to_gomips "$owrt_arch")"

	out_dir="$OUT_ROOT/$owrt_arch"
	out_bin="$out_dir/sing-box"
	mkdir -p "$out_dir"

	env_args=(CGO_ENABLED=0 GOOS=linux "GOARCH=$goarch")
	[[ -n "$goarm" ]] && env_args+=(GOARM="$goarm")
	[[ -n "$gomips" ]] && env_args+=(GOMIPS="$gomips")

	echo "==> sing-box-lite v${SING_BOX_VERSION} ${owrt_arch} (tags: ${BUILD_TAGS})"
	(
		cd "$WORKDIR"
		env "${env_args[@]}" go build -mod=mod -trimpath \
			-tags "$BUILD_TAGS" \
			-ldflags="-s -w -buildid=" \
			-o "$out_bin" ./cmd/sing-box
	)
	strip "$out_bin" 2>/dev/null || true
	hf_upx_compress "$out_bin" || true
	chmod 755 "$out_bin"
	ls -la "$out_bin"
done

echo ""
echo "Done: $OUT_ROOT"
du -sh "$OUT_ROOT"/*/* 2>/dev/null || true
