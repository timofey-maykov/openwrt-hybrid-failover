#!/usr/bin/env bash
# HF_PKG_FORMAT: ipk | apk | both (default: both)
set -euo pipefail

hf_pkg_format="${HF_PKG_FORMAT:-both}"

hf_want_ipk() {
	case "$hf_pkg_format" in
		ipk | both) return 0 ;;
		*) return 1 ;;
	esac
}

hf_want_apk() {
	case "$hf_pkg_format" in
		apk | both) return 0 ;;
		*) return 1 ;;
	esac
}
