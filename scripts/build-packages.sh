#!/usr/bin/env bash
# Build OpenWrt packages: .ipk (24.x / opkg) and .apk (25.12+ / apk).
# HF_PKG_FORMAT=ipk|apk|both (default: both)
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/arch-map.sh
source "$ROOT_DIR/scripts/lib/arch-map.sh"
# shellcheck source=scripts/lib/opkg-build.sh
source "$ROOT_DIR/scripts/lib/opkg-build.sh"
# shellcheck source=scripts/lib/apk-build.sh
source "$ROOT_DIR/scripts/lib/apk-build.sh"
# shellcheck source=scripts/lib/pkg-format.sh
source "$ROOT_DIR/scripts/lib/pkg-format.sh"
# shellcheck source=scripts/lib/compress-binary.sh
source "$ROOT_DIR/scripts/lib/compress-binary.sh"

VERSION="$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")"
PKG_RELEASE="${PKG_RELEASE:-1}"
FULL_VERSION="${VERSION}-${PKG_RELEASE}"
APK_FULL_VERSION="${VERSION}-r${PKG_RELEASE}"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
IPK_DIR="$DIST_DIR/ipk"
APK_DIR="$DIST_DIR/apk"
BIN_DIR="$DIST_DIR/binaries"
STAGE_DIR="$DIST_DIR/stage"
BOT_BIN="hybrid-failover-bot"
CORE_BIN="hybrid-failover"
BOT_SRC="$ROOT_DIR/bot"
CORE_SRC="$ROOT_DIR"

# HF_BUILD_SET: full (default) | core (hybrid-failover-core + bot only, no LuCI)
HF_BUILD_SET="${HF_BUILD_SET:-full}"

hf_build_bot() {
	[[ "$HF_BUILD_SET" == "full" || "$HF_BUILD_SET" == "core" ]]
}

hf_build_luci() {
	[[ "$HF_BUILD_SET" == "full" ]]
}

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "Missing: $1" >&2; exit 1; }; }
need_cmd go
need_cmd tar

rm -rf "$STAGE_DIR"
mkdir -p "$IPK_DIR" "$APK_DIR" "$BIN_DIR" "$STAGE_DIR"

hf_want_ipk && rm -rf "$IPK_DIR" && mkdir -p "$IPK_DIR"
hf_want_apk && rm -rf "$APK_DIR" && mkdir -p "$APK_DIR"

	echo "==> Building Hybrid Failover packages (v${FULL_VERSION}, set=${HF_BUILD_SET}, format: ${HF_PKG_FORMAT:-both}, HF_UPX=${HF_UPX:-auto})"

go_build_binary() {
	local owrt_arch="$1"
	local bin_name="$2"
	local workdir="$3"
	local pkg_path="$4"
	local out="$BIN_DIR/${owrt_arch}/${bin_name}"

	local goarch goarm gomips
	goarch="$(openwrt_arch_to_go "$owrt_arch")" || return 1
	goarm="$(openwrt_arch_to_goarm "$owrt_arch")"
	gomips="$(openwrt_arch_to_gomips "$owrt_arch")"

	mkdir -p "$(dirname "$out")"
	local env_args=(CGO_ENABLED=0 GOOS=linux "GOARCH=$goarch")
	[[ -n "$goarm" ]] && env_args+=(GOARM="$goarm")
	[[ -n "$gomips" ]] && env_args+=(GOMIPS="$gomips")

	echo "  go build ${bin_name}: ${owrt_arch} -> GOARCH=${goarch}"
	(
		cd "$workdir"
		local ld_ver
		ld_ver="$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")"
		env "${env_args[@]}" go build -mod=mod -trimpath \
			-ldflags="-s -w -buildid= -X github.com/tmaykov/openwrt-hybrid-failover/internal/version.Core=${ld_ver}" \
			-o "$out" "./${pkg_path}"
	)
	strip "$out" 2>/dev/null || true
	hf_upx_compress "$out" || return 1
	chmod 755 "$out"
}

write_control() {
	local dest="$1"
	local pkg="$2"
	local arch="$3"
	local depends="$4"
	local desc="$5"
	local size_kb="${6:-0}"
	local version="${7:-$FULL_VERSION}"

	cat >"$dest" <<EOF
Package: ${pkg}
Version: ${version}
Depends: ${depends}
Architecture: ${arch}
Installed-Size: ${size_kb}
Description: ${desc}
EOF
}

pack_pkg() {
	local pkg_root="$1"
	if hf_want_ipk; then
		opkg_build "$pkg_root" "$IPK_DIR"
	fi
	if hf_want_apk; then
		apk_build "$pkg_root" "$APK_DIR"
	fi
}

build_core_pkg() {
	local owrt_arch="$1"
	local pkg_root="$STAGE_DIR/hybrid-failover-core-${owrt_arch}"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	local bin_src="$BIN_DIR/${owrt_arch}/${CORE_BIN}"
	[[ -x "$bin_src" ]] || { echo "Missing binary: $bin_src" >&2; return 1; }

	mkdir -p "$pkg_root/usr/sbin" "$pkg_root/etc/init.d" "$pkg_root/etc/config"
	cp "$bin_src" "$pkg_root/usr/sbin/${CORE_BIN}"
	cp "$ROOT_DIR/openwrt/etc/init.d/hybrid-failover" "$pkg_root/etc/init.d/"
	cp "$ROOT_DIR/openwrt/etc/config/hybrid-failover" "$pkg_root/etc/config/"
	chmod 755 "$pkg_root/etc/init.d/hybrid-failover"
	cp "$ROOT_DIR/packages/hybrid-failover-core/CONTROL/postinst" "$pkg_root/CONTROL/"
	cp "$ROOT_DIR/packages/hybrid-failover-core/CONTROL/conffiles" "$pkg_root/CONTROL/"

	local size_kb
	size_kb="$(( ($(stat -f%z "$bin_src" 2>/dev/null || stat -c%s "$bin_src") + 1023) / 1024 ))"
	write_control "$pkg_root/CONTROL/control" "hybrid-failover-core" "$owrt_arch" \
		"sing-box ca-bundle uci procd" \
		"Hybrid Failover Core: Go routing stack for OpenWrt" \
		"$size_kb" "$FULL_VERSION"

	pack_pkg "$pkg_root"
}

build_bot_pkg() {
	local owrt_arch="$1"
	local pkg_root="$STAGE_DIR/hybrid-failover-bot-${owrt_arch}"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	local bin_src="$BIN_DIR/${owrt_arch}/${BOT_BIN}"
	[[ -x "$bin_src" ]] || { echo "Missing binary: $bin_src" >&2; return 1; }

	mkdir -p "$pkg_root/usr/bin" "$pkg_root/etc/init.d" "$pkg_root/etc/config"
	cp "$bin_src" "$pkg_root/usr/bin/${BOT_BIN}"
	cp "$BOT_SRC/openwrt/etc/init.d/${BOT_BIN}" "$pkg_root/etc/init.d/"
	chmod 755 "$pkg_root/etc/init.d/${BOT_BIN}"
	cp "$BOT_SRC/openwrt/etc/config/${BOT_BIN}" "$pkg_root/etc/config/"
	cp "$BOT_SRC/openwrt/etc/${BOT_BIN}.json" "$pkg_root/etc/"
	cp "$ROOT_DIR/packages/hybrid-failover-bot/CONTROL/postinst" "$pkg_root/CONTROL/"
	cp "$ROOT_DIR/packages/hybrid-failover-bot/CONTROL/conffiles" "$pkg_root/CONTROL/"

	local size_kb
	size_kb="$(( ($(stat -f%z "$bin_src" 2>/dev/null || stat -c%s "$bin_src") + 1023) / 1024 ))"
	write_control "$pkg_root/CONTROL/control" "hybrid-failover-bot" "$owrt_arch" \
		"libc procd ca-bundle uci hybrid-failover-core" \
		"Telegram bot for Hybrid Failover (uses core RPC)" \
		"$size_kb" "$FULL_VERSION"

	pack_pkg "$pkg_root"
}

build_luci_unified_pkg() {
	local pkg_root="$STAGE_DIR/luci-app-hybrid-failover"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL"

	local luci_root="$ROOT_DIR/luci/root"
	mkdir -p "$pkg_root/www/luci-static/resources/view/hybrid-failover"
	mkdir -p "$pkg_root/usr/share/luci/menu.d"
	mkdir -p "$pkg_root/usr/share/rpcd/acl.d"
	mkdir -p "$pkg_root/usr/share/rpcd/ucode"

	cp "$luci_root/www/luci-static/resources/view/hybrid-failover/"*.js \
		"$pkg_root/www/luci-static/resources/view/hybrid-failover/" 2>/dev/null || true
	cp "$luci_root/usr/share/luci/menu.d/luci-app-hybrid-failover.json" \
		"$pkg_root/usr/share/luci/menu.d/"
	cp "$luci_root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json" \
		"$pkg_root/usr/share/rpcd/acl.d/"
	[[ -f "$luci_root/usr/share/rpcd/ucode/hybrid-failover" ]] && \
		cp "$luci_root/usr/share/rpcd/ucode/hybrid-failover" "$pkg_root/usr/share/rpcd/ucode/"

	cp "$ROOT_DIR/packages/luci-app-hybrid-failover/CONTROL/postinst" "$pkg_root/CONTROL/"

	write_control "$pkg_root/CONTROL/control" "luci-app-hybrid-failover" "all" \
		"luci-base luci-compat luci-i18n-hybrid-failover hybrid-failover-core hybrid-failover-bot" \
		"Hybrid Failover LuCI: routing, dashboard, clients, bot" \
		"64" "$FULL_VERSION"

	pack_pkg "$pkg_root"
}

build_luci_i18n_pkg() {
	local pkg_root="$STAGE_DIR/luci-i18n-hybrid-failover"
	rm -rf "$pkg_root"
	mkdir -p "$pkg_root/CONTROL" "$pkg_root/usr/lib/lua/luci/i18n"

	if [[ ! -f "$ROOT_DIR/luci/i18n/hybrid-failover.en.lmo" ]]; then
		chmod +x "$ROOT_DIR/scripts/compile-luci-i18n.sh"
		"$ROOT_DIR/scripts/compile-luci-i18n.sh"
	fi
	[[ -f "$ROOT_DIR/luci/i18n/hybrid-failover.en.lmo" ]] || {
		echo "Missing luci/i18n/hybrid-failover.en.lmo: run scripts/compile-luci-i18n.sh" >&2
		exit 1
	}
	cp "$ROOT_DIR/luci/i18n/"*.lmo "$pkg_root/usr/lib/lua/luci/i18n/" 2>/dev/null || true
	cp "$ROOT_DIR/luci/i18n/hybrid-failover.en.lmo" "$pkg_root/usr/lib/lua/luci/i18n/"

	cp "$ROOT_DIR/packages/luci-i18n-hybrid-failover/CONTROL/postinst" "$pkg_root/CONTROL/"

	write_control "$pkg_root/CONTROL/control" "luci-i18n-hybrid-failover" "all" \
		"luci-base" \
		"Hybrid Failover LuCI translations (EN/RU)" \
		"32" "$FULL_VERSION"

	pack_pkg "$pkg_root"
}

for arch in "${HF_BUILD_ARCHS[@]}"; do
	go_build_binary "$arch" "$CORE_BIN" "$CORE_SRC" "core/cmd/hybrid-failover"
	build_core_pkg "$arch"
	if hf_build_bot && [[ -d "$ROOT_DIR/packages/hybrid-failover-bot" ]]; then
		go_build_binary "$arch" "$BOT_BIN" "$BOT_SRC" "cmd/hybrid-failover-bot"
		build_bot_pkg "$arch"
	fi
done

if hf_build_luci && [[ -d "$ROOT_DIR/luci/root" ]]; then
	echo "==> Building luci-i18n-hybrid-failover (all)"
	build_luci_i18n_pkg
	echo "==> Building luci-app-hybrid-failover (all)"
	build_luci_unified_pkg
fi

MANIFEST="$DIST_DIR/manifest.json"
{
	echo '{'
	echo "  \"version\": \"${FULL_VERSION}\","
	echo "  \"apk_version\": \"${APK_FULL_VERSION}\","
	echo "  \"pkg_format\": \"${HF_PKG_FORMAT:-both}\","
	echo '  "packages": {'
	first=1
	for dir in "$IPK_DIR" "$APK_DIR"; do
		[[ -d "$dir" ]] || continue
		for f in "$dir"/*.{ipk,apk}; do
			[[ -f "$f" ]] || continue
			base="$(basename "$f")"
			sha="$(sha256sum "$f" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$f" | awk '{print $1}')"
			[[ $first -eq 1 ]] && first=0 || echo ','
			printf '    "%s": {"sha256": "%s", "size": %s}' "$base" "$sha" "$(wc -c <"$f" | tr -d ' ')"
		done
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
hf_want_ipk && { echo "ipk: $IPK_DIR"; ls -la "$IPK_DIR" 2>/dev/null || true; }
hf_want_apk && { echo "apk: $APK_DIR"; ls -la "$APK_DIR" 2>/dev/null || true; }
echo "Manifest: $MANIFEST"
