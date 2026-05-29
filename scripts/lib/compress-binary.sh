#!/usr/bin/env bash
# Optional post-link compression for release binaries (OpenWrt overlay).
# HF_UPX=auto (default): compress when `upx` is in PATH
# HF_UPX=1: require upx
# HF_UPX=0: skip

hf_upx_compress() {
	local bin="$1"
	[[ -f "$bin" ]] || return 1

	local mode="${HF_UPX:-auto}"
	case "$mode" in
		0|false|no|off) return 0 ;;
		auto)
			command -v upx >/dev/null 2>&1 || return 0
			;;
		1|true|yes|on|best)
			command -v upx >/dev/null 2>&1 || {
				echo "HF_UPX=${mode} but upx not found (brew install upx / apt install upx-ucl)" >&2
				return 1
			}
			;;
		*)
			echo "Unknown HF_UPX=${mode} (use 0|1|auto)" >&2
			return 1
			;;
	esac

	local before after
	before="$(wc -c <"$bin" | tr -d ' ')"
	# --lzma: smaller; fallback without lzma on older upx
	if ! upx --best --lzma "$bin" >/dev/null 2>&1; then
		upx --best "$bin" >/dev/null
	fi
	after="$(wc -c <"$bin" | tr -d ' ')"
	local pct=0
	if [[ "$before" -gt 0 ]]; then
		pct=$(( (after * 100) / before ))
	fi
	echo "  upx: $(basename "$bin") ${before} -> ${after} bytes (~${pct}%)"
}
