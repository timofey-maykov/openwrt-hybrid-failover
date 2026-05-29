#!/usr/bin/env bash
# Compile luci/po/*.po → luci/i18n/*.lmo (requires po2lmo from OpenWrt luci-base).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PO2LMO="${PO2LMO:-}"
CACHE="$ROOT_DIR/.cache/luci-po2lmo"
OUT_DIR="$ROOT_DIR/luci/i18n"

if [[ -z "$PO2LMO" ]]; then
	if command -v po2lmo >/dev/null 2>&1; then
		PO2LMO="$(command -v po2lmo)"
	elif [[ -x "$CACHE/repo/modules/luci-base/src/po2lmo" ]]; then
		PO2LMO="$CACHE/repo/modules/luci-base/src/po2lmo"
	else
		echo "Building po2lmo from openwrt/luci (one-time)…" >&2
		mkdir -p "$CACHE"
		git clone --depth 1 --filter=blob:none --sparse https://github.com/openwrt/luci.git "$CACHE/repo"
		(
			cd "$CACHE/repo"
			git sparse-checkout set modules/luci-base/src
			make -C modules/luci-base/src po2lmo
		)
		PO2LMO="$CACHE/repo/modules/luci-base/src/po2lmo"
	fi
fi

mkdir -p "$OUT_DIR"
compile_one() {
	local lang="$1"
	local po="$ROOT_DIR/luci/po/$lang/hybrid-failover.po"
	local lmo="$OUT_DIR/hybrid-failover.$lang.lmo"
	[[ -f "$po" ]] || { echo "missing $po" >&2; return 1; }
	"$PO2LMO" "$po" "$lmo"
	if [[ -s "$lmo" ]]; then
		echo "wrote $lmo"
		return 0
	fi
	rm -f "$lmo"
	echo "skip $lang: no differing translations (source strings used)" >&2
	return 0
}

compile_one en
compile_one ru || true
