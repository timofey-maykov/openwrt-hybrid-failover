#!/bin/sh
# LuCI/rpcd smoke: requires hybrid-failover-core on PATH or HF_BIN.
set -eu
BIN="${HF_BIN:-/usr/sbin/hybrid-failover}"
fail() { echo "FAIL: $*" >&2; exit 1; }

command -v ubus >/dev/null 2>&1 || fail "ubus not found"
[ -x "$BIN" ] || fail "binary not found: $BIN"

for m in status health history check_nft global_check export_history; do
	echo "==> ubus hybrid-failover $m"
	out="$(ubus call hybrid-failover "$m" '{}' 2>&1)" || fail "ubus $m: $out"
	echo "$out" | head -c 200
	echo "..."
done

echo "OK: luci-ubus-smoke"
