#!/bin/ash
# Hybrid Failover: установка на OpenWrt одной командой.
#
# Полная установка (core + bot + LuCI):
#   wget -O /tmp/install.sh https://raw.githubusercontent.com/OWNER/REPO/main/scripts/install-on-router.sh \
#     && HF_REPO=OWNER/REPO ash /tmp/install.sh
#
# Только Telegram-бот:
#   HF_MODE=bot ash /tmp/install.sh
#
# Переменные:
#   HF_REPO     : GitHub owner/repo (по умолчанию timofey-maykov/openwrt-hybrid-failover)
#   HF_VERSION  : тег релиза (v1.0.0) или latest
#   HF_MODE     : full | bot
#   HF_BRANCH   : ветка для скачивания исходников (main)
#   HF_TOKEN    : токен бота (опционально, сразу в JSON)
#   HF_ADMIN_IDS: ID админов через запятую (опционально)

set -eu

HF_REPO="${HF_REPO:-timofey-maykov/openwrt-hybrid-failover}"
HF_VERSION="${HF_VERSION:-latest}"
HF_MODE="${HF_MODE:-full}"
HF_BRANCH="${HF_BRANCH:-main}"
HF_TOKEN="${HF_TOKEN:-}"
HF_ADMIN_IDS="${HF_ADMIN_IDS:-}"
GITHUB_RAW="https://raw.githubusercontent.com/${HF_REPO}/${HF_BRANCH}"
GITHUB_API="https://api.github.com/repos/${HF_REPO}"
WORKDIR="/tmp/hybrid-failover-install.$$"
DIST_CACHE="${WORKDIR}/dist"

log() { printf '[hybrid-failover] %s\n' "$*"; }
warn() { printf '[hybrid-failover] WARN: %s\n' "$*" >&2; }
die() { printf '[hybrid-failover] ERROR: %s\n' "$*" >&2; exit 1; }

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
		aarch64_generic) echo "aarch64_generic" ;;
		aarch64*|arm64*) echo "aarch64_cortex-a53" ;;
		mipsel*|mips64el*) echo "mipsel_24kc" ;;
		mips_*) echo "mips_24kc" ;;
		arm*) echo "arm_cortex-a7" ;;
		x86_64) echo "x86_64" ;;
		*) echo "$_arch" ;;
	esac
}

normalize_arch() { detect_arch; }

pkg_manager() {
	if command -v apk >/dev/null 2>&1; then
		echo apk
	elif command -v opkg >/dev/null 2>&1; then
		echo opkg
	else
		echo unknown
	fi
}

pkg_update() {
	_pm="$(pkg_manager)"
	case "$_pm" in
		apk) apk update >/dev/null 2>&1 || warn "apk update failed" ;;
		opkg) opkg update >/dev/null 2>&1 || warn "opkg update failed" ;;
		*) warn "нет apk/opkg" ;;
	esac
}

pkg_is_installed() {
	_pkg="$1"
	_pm="$(pkg_manager)"
	case "$_pm" in
		apk) apk info -e "$_pkg" 2>/dev/null | grep -q "^${_pkg}-" ;;
		opkg) opkg list-installed 2>/dev/null | grep -q "^${_pkg} " ;;
		*) return 1 ;;
	esac
}

pkg_install_file() {
	_path="$1"
	_pm="$(pkg_manager)"
	case "$_pm" in
		apk)
			apk add --allow-untrusted "$_path" 2>/dev/null || \
				apk add --force-overwrite --allow-untrusted "$_path" 2>/dev/null || true
			;;
		opkg)
			opkg install "$_path" 2>/dev/null || \
				opkg install --force-reinstall "$_path" 2>/dev/null || true
			;;
		*) return 1 ;;
	esac
}

pkg_install_named() {
	_pkg="$1"
	_pm="$(pkg_manager)"
	case "$_pm" in
		apk) apk add "$_pkg" 2>/dev/null || true ;;
		opkg) opkg install "$_pkg" 2>/dev/null || true ;;
	esac
}

install_deps() {
	_pm="$(pkg_manager)"
	log "Установка зависимостей (${_pm})..."
	pkg_update
	for pkg in curl ca-bundle wget coreutils-base64; do
		pkg_is_installed "$pkg" && continue
		pkg_install_named "$pkg" || warn "не удалось установить $pkg"
	done
	case "$HF_MODE" in
		full)
			for pkg in sing-box; do
				pkg_is_installed "$pkg" && continue
				pkg_install_named "$pkg" || warn "не удалось установить $pkg"
			done
			;;
		patches)
			die "HF_MODE=patches удалён: используйте HF_MODE=full"
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
	if [ "$HF_VERSION" != "latest" ]; then
		echo "$HF_VERSION"
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

HF_PKG_RELEASE="${HF_PKG_RELEASE:-1}"

install_pkg_url() {
	_url="$1"
	_pkg="$2"
	_name="$(basename "$_url")"
	_path="${WORKDIR}/${_name}"
	download_file_optional "$_url" "$_path" || return 1
	log "Установка $_name ..."
	pkg_install_file "$_path"
	pkg_is_installed "$_pkg" && return 0
	return 1
}

release_pkg_names() {
	_tag="$1"
	_pkg="$2"
	_arch="$3"
	_ver="$(echo "$_tag" | sed 's/^v//')"
	_rel="${HF_PKG_RELEASE}"
	_pm="$(pkg_manager)"
	case "$_pm" in
		apk)
			_base="${_pkg}-${_ver}-r${_rel}"
			if [ "$_arch" = "all" ]; then
				echo "${_base}.apk"
			else
				echo "${_base}_${_arch}.apk"
				echo "${_base}.apk"
			fi
			;;
		opkg)
			echo "${_pkg}_${_ver}-${_rel}_${_arch}.ipk"
			;;
		*)
			echo "${_pkg}_${_ver}-${_rel}_${_arch}.ipk"
			;;
	esac
}

try_install_pkg() {
	_base="https://github.com/${HF_REPO}/releases/download/$1"
	_pkg="$2"
	_arch="$3"
	for _name in $(release_pkg_names "$1" "$_pkg" "$_arch"); do
		_url="${_base}/${_name}"
		if install_pkg_url "$_url" "$_pkg"; then
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

	case "$HF_MODE" in
		bot)
			try_install_pkg "$_tag" "hybrid-failover-bot" "$_arch" || \
				die "Не найден hybrid-failover-bot для $_arch в релизе $_tag"
			try_install_pkg "$_tag" "luci-app-hybrid-failover-bot" "all" || \
				die "Не найден luci-app-hybrid-failover-bot в релизе $_tag"
			;;
		patches)
			die "HF_MODE=patches удалён: используйте HF_MODE=full (hybrid-failover-core)"
			;;
		full)
			try_install_pkg "$_tag" "hybrid-failover-core" "$_arch" || \
				die "Не найден hybrid-failover-core для $_arch в релизе $_tag"
			try_install_pkg "$_tag" "hybrid-failover-bot" "$_arch" || \
				die "Не найден hybrid-failover-bot для $_arch в релизе $_tag"
			try_install_pkg "$_tag" "luci-app-hybrid-failover" "all" || \
				try_install_pkg "$_tag" "luci-app-hybrid-failover-bot" "all" || \
				die "Не найден luci-app-hybrid-failover в релизе $_tag"
			;;
		*) die "Неизвестный HF_MODE=$HF_MODE (full|bot)" ;;
	esac
	return 0
}

install_from_local_dist() {
	_arch="$(normalize_arch)"
	_dist="${HF_DIST_DIR:-/tmp/hf-dist}"
	_pm="$(pkg_manager)"
	_sub=""
	case "$_pm" in
		apk) [ -d "$_dist/apk" ] && _sub="apk" ;;
		opkg) [ -d "$_dist/ipk" ] && _sub="ipk" ;;
	esac
	[ -n "$_sub" ] || return 1
	_dir="${_dist}/${_sub}"
	log "Установка из локального каталога $_dir (${_pm})"
	case "$HF_MODE" in
		bot|full)
			if [ "$HF_MODE" = "full" ]; then
				for _f in "${_dir}"/hybrid-failover-core_*_"${_arch}".* \
					"${_dir}"/hybrid-failover-core-*_"${_arch}".* \
					"${_dir}"/hybrid-failover-core-*."${_sub}"; do
					[ -f "$_f" ] || continue
					pkg_install_file "$_f"
					break
				done
			fi
			for _f in "${_dir}"/hybrid-failover-bot_*_"${_arch}".* "${_dir}"/hybrid-failover-bot-*_"${_arch}".* \
				"${_dir}"/hybrid-failover-bot-*."${_sub}"; do
				[ -f "$_f" ] || continue
				pkg_install_file "$_f"
				break
			done
			for _f in "${_dir}"/luci-app-hybrid-failover_*_all.* \
				"${_dir}"/luci-app-hybrid-failover-*."${_sub}" \
				"${_dir}"/luci-app-hybrid-failover-bot_*_all.* \
				"${_dir}"/luci-app-hybrid-failover-bot-*."${_sub}"; do
				[ -f "$_f" ] || continue
				pkg_install_file "$_f"
				break
			done
			;;
	esac
	return 0
}

configure_bot_json() {
	_cfg="/etc/hybrid-failover-bot.json"
	[ -f "$_cfg" ] || return 0
	_changed=0
	if [ -n "${HF_TOKEN:-}" ]; then
		sed -i "s|\"token\": *\"[^\"]*\"|\"token\": \"${HF_TOKEN}\"|" "$_cfg" 2>/dev/null && _changed=1
	fi
	if [ -n "${HF_ADMIN_IDS:-}" ]; then
		_ids="$(echo "$HF_ADMIN_IDS" | tr ',' ' ' | awk '{for(i=1;i<=NF;i++) if($i!="") print $i}' | paste -sd, -)"
		# minimal json array replace
		warn "Задайте admin_ids в LuCI или отредактируйте $_cfg вручную: $_ids"
	fi
	[ "$_changed" = 1 ] && /etc/init.d/hybrid-failover-bot restart 2>/dev/null || true
}

post_install() {
	rm -rf /tmp/luci-modulecache/* /tmp/luci-indexcache/* 2>/dev/null || true
	/etc/init.d/rpcd restart 2>/dev/null || true
	/etc/init.d/uhttpd restart 2>/dev/null || true
	configure_bot_json

	log "Готово."
	log "LuCI: http://$(uci get network.lan.ipaddr 2>/dev/null || echo ROUTER)/cgi-bin/luci/admin/services/hybrid-failover"
	log "Настройте /etc/hybrid-failover-bot.json (token, admin_ids), затем:"
	log "  uci set hybrid-failover-bot.main.enabled=1 && uci commit hybrid-failover-bot"
	log "  /etc/init.d/hybrid-failover-bot restart"
}

main() {
	need_openwrt
	ARCH="$(normalize_arch)"
	log "Hybrid Failover installer"
	log "Репозиторий: ${HF_REPO}, режим: ${HF_MODE}, arch: ${ARCH}, pkg: $(pkg_manager)"

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
