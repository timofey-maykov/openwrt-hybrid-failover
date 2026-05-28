#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROUTER_HOST="${1:-192.168.42.1}"
ROUTER_USER="${ROUTER_USER:-root}"
SSH_TARGET="${ROUTER_USER}@${ROUTER_HOST}"
SSH_OPTS=( -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new )

PODKOP_BIN="$ROOT_DIR/packaging/hybrid-failover-patch/usr/bin/podkop"
FACADE_SH="$ROOT_DIR/vendor/sing_box_config_facade.sh"
AMNEZIA_PY="$ROOT_DIR/scripts/amnezia_vpn_uri_to_vless.py"
LUCI_SECTION_JS="$ROOT_DIR/luci/section.js"
MAIN_PATCH="$ROOT_DIR/patches/main-js-dashboard-vpn-failover.patch"

for f in "$PODKOP_BIN" "$FACADE_SH" "$AMNEZIA_PY" "$LUCI_SECTION_JS" "$MAIN_PATCH"; do
  [[ -f "$f" ]] || { echo "Missing file: $f" >&2; exit 1; }
done

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "Missing command: $1" >&2; exit 1; }; }
need_cmd ssh
need_cmd base64
need_cmd patch

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

send_file_base64() {
  local local_file="$1"
  local remote_file="$2"
  local remote_mode="${3:-644}"
  base64 < "$local_file" | ssh "${SSH_OPTS[@]}" "$SSH_TARGET" \
    "base64 -d > '$remote_file' && chmod $remote_mode '$remote_file'"
}

echo "[1/7] Router info + install deps"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" '
set -e
uname -a | head -n1
opkg update >/dev/null 2>&1 || true
# Minimal runtime deps for podkop + vpn:// support
for pkg in sing-box jq coreutils-base64 curl python3-light; do
  if ! opkg status "$pkg" 2>/dev/null | grep -q "^Status:.* installed"; then
    echo "Installing $pkg ..."
    opkg install "$pkg" >/dev/null 2>&1 || echo "WARN: failed to install $pkg"
  fi
done
mkdir -p /usr/lib/podkop /etc/sing-box
'

echo "[2/7] Backup current router files"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" '
set -e
TS="$(date +%Y%m%d-%H%M%S)"
BK="/root/podkop-hf-backup-$TS"
mkdir -p "$BK"
cp -a /usr/bin/podkop "$BK/" 2>/dev/null || true
cp -a /usr/lib/podkop/sing_box_config_facade.sh "$BK/" 2>/dev/null || true
cp -a /www/luci-static/resources/view/podkop/section.js "$BK/" 2>/dev/null || true
cp -a /www/luci-static/resources/view/podkop/main.js "$BK/" 2>/dev/null || true
echo "Backup: $BK"
'

echo "[3/7] Upload podkop backend and Amnezia decoder"
send_file_base64 "$PODKOP_BIN" "/usr/bin/podkop" 755
send_file_base64 "$FACADE_SH" "/usr/lib/podkop/sing_box_config_facade.sh" 644
send_file_base64 "$AMNEZIA_PY" "/usr/lib/podkop/amnezia_vpn_uri_to_vless.py" 644

echo "[4/7] Upload LuCI section.js"
send_file_base64 "$LUCI_SECTION_JS" "/www/luci-static/resources/view/podkop/section.js" 644

echo "[5/7] Fetch and patch LuCI main.js locally"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" "cat /www/luci-static/resources/view/podkop/main.js" > "$TMP_DIR/main.js"
if grep -q "failoverOn" "$TMP_DIR/main.js"; then
  echo "LuCI main.js already contains dashboard failover patch, skip patch step"
else
  patch -p0 "$TMP_DIR/main.js" < "$MAIN_PATCH" >/dev/null
fi

echo "[6/7] Upload patched main.js"
send_file_base64 "$TMP_DIR/main.js" "/www/luci-static/resources/view/podkop/main.js" 644

echo "[7/7] Runtime tune + restart + verify"
ssh "${SSH_OPTS[@]}" "$SSH_TARGET" '
set -e
uci set podkop.settings.cache_path="/etc/sing-box/cache.db"
uci commit podkop
/etc/init.d/podkop restart
sleep 2

printf "schema="; uci get podkop.settings.config_schema_version 2>/dev/null || echo missing
printf "cache_path="; uci get podkop.settings.cache_path 2>/dev/null || echo missing
printf "python3="; command -v python3 || echo missing
printf "facade_vpn_branch="; grep -c "^    vpn)" /usr/lib/podkop/sing_box_config_facade.sh || true
printf "dashboard_patch_marker="; grep -c "failoverOn" /www/luci-static/resources/view/podkop/main.js || true
/etc/init.d/podkop status 2>/dev/null || true
'

echo "Done. Hard refresh LuCI page (Ctrl+Shift+R)."
