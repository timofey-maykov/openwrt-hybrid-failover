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

echo "==> ubus hybrid-failover switch_proxy"
active="$(ubus call hybrid-failover status '{}' 2>/dev/null | jsonfilter -e '@.data.active_outbound' 2>/dev/null || true)"
target="glob-urltest-out"
if [ "$active" = "glob-urltest-out" ]; then
	target="glob-awg-out"
fi
out="$(ubus call hybrid-failover switch_proxy "{\"section\":\"glob\",\"outbound\":\"$target\"}" 2>&1)" || fail "ubus switch_proxy: $out"
echo "$out" | head -c 300
echo
case "$out" in
	*"\"ok\": true"*) ;;
	*) fail "switch_proxy response not ok: $out" ;;
esac

echo "OK: luci-ubus-smoke"
