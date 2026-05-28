#!/usr/bin/env bash
# Build OpenWrt .ipk packages for all supported architectures (host with Go).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/arch-map.sh
source "$ROOT_DIR/scripts/lib/arch-map.sh"
# shellcheck source=scripts/lib/opkg-build.sh
source "$ROOT_DIR/scripts/lib/opkg-build.sh"

VERSION="$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")"
PKG_RELEASE="${PKG_RELEASE:-1}"
FULL_VERSION="${VERSION}-${PKG_RELEASE}"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
IPK_DIR="$DIST_DIR/ipk"
BIN_DIR="$DIST_DIR/binaries"
STAGE_DIR="$DIST_DIR/stage"
BOT_SRC="$ROOT_DIR/bot"
BIN_NAME="hybrid-failover-bot"

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "Missing: $1" >&2; exit 1; }; }
need_cmd go
need_cmd tar

rm -rf "$STAGE_DIR"
mkdir -p "$IPK_DIR" "$BIN_DIR" "$STAGE_DIR"

echo "==> Building ${BIN_NAME} for ${#HF_BUILD_ARCHS[@]} architectures (v${FULL_VERSION})"

build_bot_binary() {
	local owrt_arch="$1"
	local goarch goarm gomips
	goarch="$(openwrt_arch_to_go "$owrt_arch")" || return 1
	goarm="$(openwrt_arch_to_goarm "$owrt_arch")"
	gomips="$(openwrt_arch_to_gomips "$owrt_arch")"

	local out="$BIN_DIR/${owrt_arch}/${BIN_NAME}"
	mkdir -p "$(dirname "$out")"

	local env_args=(CGO_ENABLED=0 GOOS=linux "GOARCH=$goarch")
	[[ -n "$goarm" ]] && env_args+=(GOARM="$goarm")
	[[ -n "$gomips" ]] && env_args+=(GOMIPS="$gomips")

	echo "  go build: ${owrt_arch} -> GOARCH=${goarch} GOARM=${goarm:-'-'} GOMIPS=${gomips:-'-'}"
	(
		cd "$BOT_SRC"
		env "${env_args[@]}" go build -trimpath -ldflags="-s -w" \
			-o "$out" ./cmd/hybrid-failover-bot
	)
	strip "$out" 2>/dev/null || true
	chmod 755 "$out"
}

write_control() {
	local dest="$1"
	local pkg="$2"
	local arch="$3"
	local depends="$4"
	local desc="$5"
	local size_kb="${6:-0}"

	cat >"$dest" <<EOF
Package: ${pkg}
Version: ${FULL_VERSION}
Depends: ${depends}
Architecture: ${arch}
Installed-Size: ${size_kb}
Description: ${desc}
EOF
}

stage_copy_tree() {
	local src="$1"
	local dst="$2"
	if [[ -d "$src" ]]; then
		mkdir -p "$dst"
		cp -a "$src/." "$dst/"
	fi
}

build_bot_ipk() {
	local owrt_arch="$1"
	local pkg_root="$STAGE_DIR/hybrid-failover-bot-${owrt_arch}"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	local bin_src="$BIN_DIR/${owrt_arch}/${BIN_NAME}"
	[[ -x "$bin_src" ]] || { echo "Missing binary: $bin_src" >&2; return 1; }

	mkdir -p "$pkg_root/usr/bin"
	cp "$bin_src" "$pkg_root/usr/bin/${BIN_NAME}"

	mkdir -p "$pkg_root/etc/init.d" "$pkg_root/etc/config"
	cp "$BOT_SRC/openwrt/etc/init.d/${BIN_NAME}" "$pkg_root/etc/init.d/"
	cp "$BOT_SRC/openwrt/etc/config/${BIN_NAME}" "$pkg_root/etc/config/"
	cp "$BOT_SRC/openwrt/etc/${BIN_NAME}.json" "$pkg_root/etc/"

	cp "$ROOT_DIR/packages/hybrid-failover-bot/CONTROL/postinst" "$pkg_root/CONTROL/"
	cp "$ROOT_DIR/packages/hybrid-failover-bot/CONTROL/conffiles" "$pkg_root/CONTROL/"

	local size_kb
	size_kb="$(( ($(stat -f%z "$bin_src" 2>/dev/null || stat -c%s "$bin_src") + 1023) / 1024 ))"
	write_control "$pkg_root/CONTROL/control" "hybrid-failover-bot" "$owrt_arch" \
		"libc procd ca-bundle uci" \
		"Telegram bot for Hybrid Failover on OpenWrt (manages upstream podkop UCI)" \
		"$size_kb"

	opkg_build "$pkg_root" "$IPK_DIR"
}

build_luci_ipk() {
	local pkg_root="$STAGE_DIR/luci-app-hybrid-failover-bot"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	local luci_root="$ROOT_DIR/luci/root"
	mkdir -p "$pkg_root/www/luci-static/resources/view/hybrid-failover"
	mkdir -p "$pkg_root/usr/share/luci/menu.d"
	mkdir -p "$pkg_root/usr/share/rpcd/acl.d"

	cp "$luci_root/www/luci-static/resources/view/hybrid-failover/bot.js" \
		"$pkg_root/www/luci-static/resources/view/hybrid-failover/"
	cp "$luci_root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json" \
		"$pkg_root/usr/share/luci/menu.d/"
	cp "$luci_root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json" \
		"$pkg_root/usr/share/rpcd/acl.d/"

	cp "$ROOT_DIR/packages/luci-app-hybrid-failover-bot/CONTROL/postinst" "$pkg_root/CONTROL/"

	write_control "$pkg_root/CONTROL/control" "luci-app-hybrid-failover-bot" "all" \
		"luci-base luci-compat hybrid-failover-bot" \
		"LuCI page for Hybrid Failover Telegram bot (Russian UI)" \
		"32"

	opkg_build "$pkg_root" "$IPK_DIR"
}

build_hybrid_ipk() {
	local pkg_root="$STAGE_DIR/hybrid-failover-patch"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	mkdir -p "$pkg_root/usr/bin" "$pkg_root/usr/lib/podkop"
	mkdir -p "$pkg_root/www/luci-static/resources/view/podkop"

	cp "$ROOT_DIR/packaging/hybrid-failover-patch/usr/bin/podkop" "$pkg_root/usr/bin/podkop"
	cp "$ROOT_DIR/vendor/sing_box_config_facade.sh" "$pkg_root/usr/lib/podkop/sing_box_config_facade.sh"
	cp "$ROOT_DIR/scripts/amnezia_vpn_uri_to_vless.py" "$pkg_root/usr/lib/podkop/amnezia_vpn_uri_to_vless.py"
	cp "$ROOT_DIR/luci/section.js" "$pkg_root/www/luci-static/resources/view/podkop/section.js"

	# main.js dashboard patch is applied on router by install-on-router.sh (needs stock luci-app-podkop)
	chmod 755 "$pkg_root/usr/bin/podkop"
	chmod 644 "$pkg_root/usr/lib/podkop/"* "$pkg_root/www/luci-static/resources/view/podkop/"* 2>/dev/null || true

	cp "$ROOT_DIR/packages/hybrid-failover-patch/CONTROL/postinst" "$pkg_root/CONTROL/"

	write_control "$pkg_root/CONTROL/control" "hybrid-failover-patch" "all" \
		"podkop sing-box jq curl python3-light coreutils-base64" \
		"VPN+failover patches for upstream podkop: urltest, vpn://, LuCI section.js" \
		"256"

	opkg_build "$pkg_root" "$IPK_DIR"
}

for arch in "${HF_BUILD_ARCHS[@]}"; do
	build_bot_binary "$arch"
	build_bot_ipk "$arch"
done

echo "==> Building luci-app-hybrid-failover-bot (all)"
build_luci_ipk

echo "==> Building hybrid-failover-patch (all)"
build_hybrid_ipk

# Manifest for installer
MANIFEST="$DIST_DIR/manifest.json"
{
	echo '{'
	echo "  \"version\": \"${FULL_VERSION}\","
	echo '  "packages": {'
	first=1
	for f in "$IPK_DIR"/*.ipk; do
		[[ -f "$f" ]] || continue
		base="$(basename "$f")"
		sha="$(sha256sum "$f" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$f" | awk '{print $1}')"
		[[ $first -eq 1 ]] && first=0 || echo ','
		printf '    "%s": {"sha256": "%s", "size": %s}' "$base" "$sha" "$(wc -c <"$f" | tr -d ' ')"
	done
	echo
	echo '  },'
	echo '  "architectures": ['
	printf '    "%s"' "${HF_BUILD_ARCHS[0]}"
	for arch in "${HF_BUILD_ARCHS[@]:1}"; do
		printf ',\n    "%s"' "$arch"
	done
	echo
	echo '  ]'
	echo '}'
} >"$MANIFEST"

echo ""
echo "Done. Packages in: $IPK_DIR"
ls -la "$IPK_DIR"
echo "Manifest: $MANIFEST"
