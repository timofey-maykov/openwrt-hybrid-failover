#!/usr/bin/env bash
# OpenWrt DISTRIB_ARCH <-> Go cross-compile mapping.
# shellcheck disable=SC2034

# Architectures we build release packages for.
HF_BUILD_ARCHS=(
	aarch64_cortex-a53
	arm_cortex-a7
	mipsel_24kc
	mips_24kc
	x86_64
)

openwrt_arch_to_go() {
	local owrt_arch="$1"
	case "$owrt_arch" in
		aarch64*|arm64*) echo "arm64" ;;
		mipsel*|mips64el*) echo "mipsle" ;;
		mips_*) echo "mips" ;;
		arm*) echo "arm" ;;
		x86_64) echo "amd64" ;;
		i386|i686) echo "386" ;;
		*) return 1 ;;
	esac
}

openwrt_arch_to_goarm() {
	local owrt_arch="$1"
	case "$owrt_arch" in
		arm_cortex-a7*|arm_cortex-a8*|arm_cortex-a9*) echo "7" ;;
		arm_cortex-a5*) echo "5" ;;
		*) echo "" ;;
	esac
}

openwrt_arch_to_gomips() {
	local owrt_arch="$1"
	case "$owrt_arch" in
		mips*|mipsel*|mips64*) echo "softfloat" ;;
		*) echo "" ;;
	esac
}

# Map router DISTRIB_ARCH to nearest release package arch.
normalize_openwrt_arch() {
	local raw="${1:-}"
	case "$raw" in
		aarch64*|arm64*) echo "aarch64_cortex-a53" ;;
		mipsel*|mips64el*) echo "mipsel_24kc" ;;
		mips_*) echo "mips_24kc" ;;
		arm*) echo "arm_cortex-a7" ;;
		x86_64) echo "x86_64" ;;
		*) echo "$raw" ;;
	esac
}

detect_openwrt_arch() {
	if [[ -f /etc/openwrt_release ]]; then
		# shellcheck disable=SC1091
		. /etc/openwrt_release
		normalize_openwrt_arch "${DISTRIB_ARCH:-unknown}"
		return 0
	fi
	local u
	u="$(uname -m 2>/dev/null || echo unknown)"
	case "$u" in
		aarch64) echo "aarch64_cortex-a53" ;;
		armv7l|armv7) echo "arm_cortex-a7" ;;
		mips) echo "mips_24kc" ;;
		mipsel) echo "mipsel_24kc" ;;
		x86_64) echo "x86_64" ;;
		*) echo "unknown" ;;
	esac
}
