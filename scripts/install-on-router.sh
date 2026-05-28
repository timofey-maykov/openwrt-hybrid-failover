#!/bin/ash
# Podkop Hybrid Failover: установка на OpenWrt одной командой.
#
# Полная установка (бот + LuCI + патчи Podkop):
#   wget -O /tmp/install.sh https://raw.githubusercontent.com/OWNER/REPO/main/scripts/install-on-router.sh \
#     && PODKOP_HF_REPO=OWNER/REPO ash /tmp/install.sh
#
# Только Telegram-бот и LuCI:
#   PODKOP_HF_MODE=bot ash /tmp/install.sh
#
# Переменные:
#   PODKOP_HF_REPO     : GitHub owner/repo (по умолчанию timofey-maykov/podkop-hybrid-failover)
#   PODKOP_HF_VERSION  : тег релиза (v1.0.0) или latest
#   PODKOP_HF_MODE     : full | bot | patches
#   PODKOP_HF_BRANCH   : ветка для скачивания исходников (main)
#   PODKOP_HF_TOKEN    : токен бота (опционально, сразу в JSON)
#   PODKOP_HF_ADMIN_IDS: ID админов через запятую (опционально)

set -eu

PODKOP_HF_REPO="${PODKOP_HF_REPO:-timofey-maykov/podkop-hybrid-failover}"
PODKOP_HF_VERSION="${PODKOP_HF_VERSION:-latest}"
PODKOP_HF_MODE="${PODKOP_HF_MODE:-full}"
PODKOP_HF_BRANCH="${PODKOP_HF_BRANCH:-main}"
GITHUB_RAW="https://raw.githubusercontent.com/${PODKOP_HF_REPO}/${PODKOP_HF_BRANCH}"
GITHUB_API="https://api.github.com/repos/${PODKOP_HF_REPO}"
WORKDIR="/tmp/podkop-hf-install.$$"
DIST_CACHE="${WORKDIR}/dist"

log() { printf '[podkop-hf] %s\n' "$*"; }
warn() { printf '[podkop-hf] WARN: %s\n' "$*" >&2; }
die() { printf '[podkop-hf] ERROR: %s\n' "$*" >&2; exit 1; }

cleanup() { rm -rf "$WORKDIR" 2>/dev/null || true; }
trap cleanup EXIT

need_openwrt() {
	[ -f /etc/openwrt_release ] || die "Скрипт рассчитан на OpenWrt (/etc/openwrt_release не найден)"
}

detect_arch() {
	if [ -f /etc/openwrt_release ]; then
		# shellcheck disable=SC1091
		. /etc/openwrt_release
		_arch="${DISTRIB_ARCH:-unknown}"
	else
		_arch="$(uname -m)"
	fi
	case "$_arch" in
		aarch64*|arm64*) echo "aarch64_cortex-a53" ;;
		mipsel*|mips64el*) echo "mipsel_24kc" ;;
		mips_*) echo "mips_24kc" ;;
		arm*) echo "arm_cortex-a7" ;;
		x86_64) echo "x86_64" ;;
		*) echo "$_arch" ;;
	esac
}

normalize_arch() { detect_arch; }

install_deps() {
	log "Установка зависимостей opkg..."
	opkg update >/dev/null 2>&1 || warn "opkg update failed"
	for pkg in curl ca-bundle wget coreutils-base64; do
		opkg list-installed 2>/dev/null | grep -q "^${pkg} " && continue
		opkg install "$pkg" >/dev/null 2>&1 || warn "не удалось установить $pkg"
	done
	case "$PODKOP_HF_MODE" in
		full|patches)
			for pkg in sing-box jq python3-light patch; do
				opkg list-installed 2>/dev/null | grep -q "^${pkg} " && continue
				opkg install "$pkg" >/dev/null 2>&1 || warn "не удалось установить $pkg"
			done
			;;
	esac
}

download_file() {
	_url="$1"
	_out="$2"
	download_file_optional "$_url" "$_out" || die "Не удалось скачать: $_url"
}

download_file_optional() {
	_url="$1"
	_out="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --connect-timeout 30 --retry 3 -o "$_out" "$_url" 2>/dev/null
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$_out" "$_url" 2>/dev/null
	else
		return 1
	fi
}

resolve_release_tag() {
	if [ "$PODKOP_HF_VERSION" != "latest" ]; then
		echo "$PODKOP_HF_VERSION"
		return 0
	fi
	_tag=""
	if command -v curl >/dev/null 2>&1; then
		_tag="$(curl -fsSL "${GITHUB_API}/releases/latest" 2>/dev/null | \
			sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
	fi
	[ -n "$_tag" ] || _tag="v$(download_file "${GITHUB_RAW}/VERSION" /dev/stdout 2>/dev/null | tr -d '\r\n' || echo 1.0.0)"
	echo "$_tag"
}

install_ipk_url() {
	_url="$1"
	_name="$(basename "$_url")"
	_path="${WORKDIR}/${_name}"
	_pkg="$(echo "$_name" | sed 's/_.*//')"
	download_file_optional "$_url" "$_path" || return 1
	log "Установка $_name ..."
	opkg install "$_path" 2>/dev/null || opkg install --force-reinstall "$_path" 2>/dev/null || true
	opkg list-installed 2>/dev/null | grep -q "^${_pkg} " && return 0
	return 1
}

try_install_ipk() {
	_ver="$(echo "$1" | sed 's/^v//')"
	_base="https://github.com/${PODKOP_HF_REPO}/releases/download/$1"
	_pkg="$2"
	_arch="$3"
	for _name in "${_pkg}_${_ver}-1_${_arch}.ipk"; do
		_url="${_base}/${_name}"
		if install_ipk_url "$_url"; then
			return 0
		fi
	done
	return 1
}

install_from_release() {
	_arch="$(normalize_arch)"
	_tag="$(resolve_release_tag)"
	log "Релиз: $_tag, архитектура: $_arch"
	_ver="$(echo "$_tag" | sed 's/^v//')"

	case "$PODKOP_HF_MODE" in
		bot)
			try_install_ipk "$_tag" "podkop-telegram-bot" "$_arch" || \
				die "Не найден podkop-telegram-bot для $_arch в релизе $_tag"
			try_install_ipk "$_tag" "luci-app-podkop-bot" "all" || \
				die "Не найден luci-app-podkop-bot в релизе $_tag"
			;;
		patches)
			try_install_ipk "$_tag" "podkop-hybrid-failover" "all" || apply_hybrid_from_source
			;;
		full)
			try_install_ipk "$_tag" "podkop-telegram-bot" "$_arch" || \
				die "Не найден podkop-telegram-bot для $_arch в релизе $_tag"
			try_install_ipk "$_tag" "luci-app-podkop-bot" "all" || \
				die "Не найден luci-app-podkop-bot в релизе $_tag"
			try_install_ipk "$_tag" "podkop-hybrid-failover" "all" || apply_hybrid_from_source
			;;
		*) die "Неизвестный PODKOP_HF_MODE=$PODKOP_HF_MODE (full|bot|patches)" ;;
	esac
	return 0
}

apply_hybrid_from_source() {
	log "Применение патчей Podkop из GitHub (${PODKOP_HF_BRANCH})..."

	_backup="/root/podkop-hf-backup-$(date +%Y%m%d-%H%M%S)"
	mkdir -p "$_backup"
	cp -a /usr/bin/podkop "$_backup/" 2>/dev/null || true
	cp -a /usr/lib/podkop/sing_box_config_facade.sh "$_backup/" 2>/dev/null || true
	cp -a /www/luci-static/resources/view/podkop/section.js "$_backup/" 2>/dev/null || true
	cp -a /www/luci-static/resources/view/podkop/main.js "$_backup/" 2>/dev/null || true
	log "Резервная копия: $_backup"

	download_file "${GITHUB_RAW}/packaging/podkop-hybrid-failover/usr/bin/podkop" /usr/bin/podkop
	chmod 755 /usr/bin/podkop
	mkdir -p /usr/lib/podkop /etc/sing-box
	download_file "${GITHUB_RAW}/vendor/sing_box_config_facade.sh" /usr/lib/podkop/sing_box_config_facade.sh
	download_file "${GITHUB_RAW}/scripts/amnezia_vpn_uri_to_vless.py" /usr/lib/podkop/amnezia_vpn_uri_to_vless.py
	chmod 644 /usr/lib/podkop/sing_box_config_facade.sh /usr/lib/podkop/amnezia_vpn_uri_to_vless.py
	mkdir -p /www/luci-static/resources/view/podkop
	download_file "${GITHUB_RAW}/luci/section.js" /www/luci-static/resources/view/podkop/section.js

	if [ -f /www/luci-static/resources/view/podkop/main.js ]; then
		if grep -q failoverOn /www/luci-static/resources/view/podkop/main.js 2>/dev/null; then
			log "main.js уже содержит патч дашборда"
		elif command -v patch >/dev/null 2>&1; then
			download_file "${GITHUB_RAW}/patches/main-js-dashboard-vpn-failover.patch" "${WORKDIR}/main.patch"
			cp /www/luci-static/resources/view/podkop/main.js "${WORKDIR}/main.js"
			cd "${WORKDIR}" && patch -p0 main.js < main.patch >/dev/null 2>&1 && \
				cp main.js /www/luci-static/resources/view/podkop/main.js && \
				log "Патч main.js применён" || warn "не удалось применить патч main.js"
		else
			warn "patch не установлен: дашборд VPN+failover может не отображаться"
		fi
	else
		warn "luci-app-podkop не найден: пропуск main.js"
	fi

	uci set podkop.settings.cache_path='/etc/sing-box/cache.db' 2>/dev/null || true
	uci commit podkop 2>/dev/null || true
	/etc/init.d/podkop restart 2>/dev/null || true
}

install_from_local_dist() {
	# Если скрипт запущен из клонированного репозитория на роутере (редко)
	_arch="$(normalize_arch)"
	_dist="${PODKOP_HF_DIST_DIR:-/tmp/podkop-hf-dist}"
	[ -d "$_dist/ipk" ] || return 1
	log "Установка из локального каталога $_dist/ipk"
	case "$PODKOP_HF_MODE" in
		bot|full)
			opkg install "${_dist}/ipk"/podkop-telegram-bot_*_"${_arch}".ipk
			opkg install "${_dist}/ipk"/luci-app-podkop-bot_*_all.ipk
			;;
	esac
	case "$PODKOP_HF_MODE" in
		full|patches)
			opkg install "${_dist}/ipk"/podkop-hybrid-failover_*_all.ipk 2>/dev/null || apply_hybrid_from_source
			;;
	esac
	return 0
}

configure_bot_json() {
	_cfg="/etc/podkop-telegram-bot.json"
	[ -f "$_cfg" ] || return 0
	_changed=0
	if [ -n "${PODKOP_HF_TOKEN:-}" ]; then
		sed -i "s|\"token\": *\"[^\"]*\"|\"token\": \"${PODKOP_HF_TOKEN}\"|" "$_cfg" 2>/dev/null && _changed=1
	fi
	if [ -n "${PODKOP_HF_ADMIN_IDS:-}" ]; then
		_ids="$(echo "$PODKOP_HF_ADMIN_IDS" | tr ',' ' ' | awk '{for(i=1;i<=NF;i++) if($i!="") print $i}' | paste -sd, -)"
		# minimal json array replace
		warn "Задайте admin_ids в LuCI или отредактируйте $_cfg вручную: $_ids"
	fi
	[ "$_changed" = 1 ] && /etc/init.d/podkop-telegram-bot restart 2>/dev/null || true
}

post_install() {
	rm -rf /tmp/luci-modulecache/* /tmp/luci-indexcache/* 2>/dev/null || true
	/etc/init.d/rpcd restart 2>/dev/null || true
	/etc/init.d/uhttpd restart 2>/dev/null || true
	configure_bot_json

	log "Готово."
	log "LuCI бот: http://$(uci get network.lan.ipaddr 2>/dev/null || echo ROUTER)/cgi-bin/luci/admin/services/podkop-bot"
	log "Настройте /etc/podkop-telegram-bot.json (token, admin_ids), затем:"
	log "  uci set podkop-telegram-bot.main.enabled=1 && uci commit podkop-telegram-bot"
	log "  /etc/init.d/podkop-telegram-bot restart"
}

main() {
	need_openwrt
	ARCH="$(normalize_arch)"
	log "Podkop Hybrid Failover installer"
	log "Репозиторий: ${PODKOP_HF_REPO}, режим: ${PODKOP_HF_MODE}, arch: ${ARCH}"

	mkdir -p "$WORKDIR"
	install_deps

	if install_from_local_dist; then
		post_install
		exit 0
	fi

	install_from_release
	post_install
}

main "$@"
