#!/usr/bin/env bash
# Build OpenWrt 25.12+ compatible .apk (Alpine Package Keeper / apk-tools v3).
# Uses `apk mkpkg` from apk-tools 3.x (Alpine edge or OpenWrt SDK host tools).
set -euo pipefail

APK_DOCKER_IMAGE="${APK_DOCKER_IMAGE:-alpine:edge}"

_apk_bin() {
	if [[ -n "${APK_BIN:-}" && -x "${APK_BIN}" ]]; then
		echo "${APK_BIN}"
		return 0
	fi
	if command -v apk >/dev/null 2>&1 && apk mkpkg 2>&1 | grep -q "required info field"; then
		command -v apk
		return 0
	fi
	return 1
}

# 1.0.5-1 -> 1.0.5-r1 (OpenWrt APK version schema)
to_apk_version() {
	local v="$1"
	if [[ "$v" =~ -r[0-9]+$ ]]; then
		echo "$v"
	elif [[ "$v" =~ ^(.+)-([0-9]+)$ ]]; then
		echo "${BASH_REMATCH[1]}-r${BASH_REMATCH[2]}"
	else
		echo "$v"
	fi
}

_apk_arch() {
	case "$1" in
		all) echo "noarch" ;;
		*) echo "$1" ;;
	esac
}

_write_apk_post_install() {
	local postinst="${1:-}"
	local dest="$2"
	local pkg_name="$3"

	{
		echo '#!/bin/sh'
		echo '[ "${IPKG_NO_SCRIPT}" = "1" ] && exit 0'
		echo '[ -s "${IPKG_INSTROOT}/lib/functions.sh" ] && . "${IPKG_INSTROOT}/lib/functions.sh"'
		echo "export root=\"\${IPKG_INSTROOT}\""
		echo "export pkgname=\"${pkg_name}\""
		if [[ -f "$postinst" ]]; then
			sed '/^#!/d' "$postinst"
		fi
		echo '[ -s "${IPKG_INSTROOT}/lib/functions.sh" ] && default_postinst'
	} >"$dest"
	chmod 755 "$dest"
}

_run_apk_mkpkg_native() {
	local apk_bin="$1"
	shift
	"$apk_bin" mkpkg "$@"
}

_run_apk_mkpkg_docker() {
	local idir="$1"
	local scripts_dir="$2"
	local out_dir="$3"
	local out_name="$4"
	shift 4
	local -a mkpkg_args=("$@")

	command -v docker >/dev/null 2>&1 || {
		echo "apk mkpkg not found and docker unavailable (need ${APK_DOCKER_IMAGE})" >&2
		return 1
	}

	local args_quoted=""
	local arg
	for arg in "${mkpkg_args[@]}"; do
		args_quoted+=" $(printf '%q' "$arg")"
	done

	docker run --rm \
		-v "${idir}:/pkg:ro" \
		-v "${scripts_dir}:/scripts:ro" \
		-v "${out_dir}:/out" \
		"${APK_DOCKER_IMAGE}" \
		sh -c "apk add -q apk-tools 2>/dev/null || true; apk mkpkg${args_quoted} --script post-install:/scripts/post-install --files /pkg --output /out/${out_name}"
}

apk_build() {
	local pkg_root="$1"
	local out_dir="$2"
	local control="$pkg_root/CONTROL/control"

	[[ -f "$control" ]] || { echo "Missing CONTROL/control in $pkg_root" >&2; return 1; }

	local pkg_name pkg_version pkg_arch
	pkg_name="$(awk -F': ' '/^Package:/{print $2; exit}' "$control")"
	pkg_version="$(awk -F': ' '/^Version:/{print $2; exit}' "$control")"
	pkg_arch="$(awk -F': ' '/^Architecture:/{print $2; exit}' "$control")"
	local depends desc license
	depends="$(awk -F': ' '/^Depends:/{print $2; exit}' "$control")"
	desc="$(awk -F': ' '/^Description:/{print $2; exit}' "$control")"
	license="$(awk -F': ' '/^License:/{print $2; exit}' "$control")"
	license="${license:-GPL-2.0}"

	[[ -n "$pkg_name" && -n "$pkg_version" && -n "$pkg_arch" ]] || {
		echo "Invalid control file: $control" >&2
		return 1
	}

	local apk_version apk_arch out_name
	apk_version="$(to_apk_version "$pkg_version")"
	apk_arch="$(_apk_arch "$pkg_arch")"
	out_name="${pkg_name}-${apk_version}"
	if [[ "$apk_arch" != "noarch" ]]; then
		out_name="${out_name}_${pkg_arch}"
	fi
	out_name="${out_name}.apk"

	mkdir -p "$out_dir"
	local idir scripts_dir
	idir="$(mktemp -d)"
	scripts_dir="$(mktemp -d)"

	# Staging tree (no CONTROL/)
	if command -v rsync >/dev/null 2>&1; then
		rsync -a --exclude=CONTROL/ "$pkg_root/" "$idir/"
	else
		(
			cd "$pkg_root"
			tar cf - --exclude=CONTROL . | tar xf - -C "$idir"
		)
	fi

	mkdir -p "$idir/lib/apk/packages"
	(
		cd "$idir"
		find . \( -type f -o -type l \) ! -path './lib/apk/packages/*' |
			sed 's|^\./|/|' | sort
	) >"$idir/lib/apk/packages/${pkg_name}.list"

	if [[ -f "$pkg_root/CONTROL/conffiles" ]]; then
		cp "$pkg_root/CONTROL/conffiles" "$idir/lib/apk/packages/${pkg_name}.conffiles"
	fi

	_write_apk_post_install "$pkg_root/CONTROL/postinst" \
		"$scripts_dir/post-install" "$pkg_name"

	local -a mkpkg_args=(
		--info "name:${pkg_name}"
		--info "version:${apk_version}"
		--info "description:${desc:-Hybrid Failover}"
		--info "arch:${apk_arch}"
		--info "license:${license}"
		--info "origin:openwrt-hybrid-failover"
	)
	[[ -n "$depends" ]] && mkpkg_args+=(--info "depends:${depends}")

	rm -f "$out_dir/$out_name"
	if _apk_bin >/dev/null 2>&1; then
		local apk_bin
		apk_bin="$(_apk_bin)"
		_run_apk_mkpkg_native "$apk_bin" "${mkpkg_args[@]}" \
			--script "post-install:${scripts_dir}/post-install" \
			--files "$idir" \
			--output "$out_dir/$out_name"
	else
		_run_apk_mkpkg_docker "$idir" "$scripts_dir" "$out_dir" "$out_name" "${mkpkg_args[@]}"
	fi

	[[ -f "$out_dir/$out_name" ]] || {
		rm -rf "$idir" "$scripts_dir"
		echo "apk mkpkg failed: $out_name" >&2
		return 1
	}
	rm -rf "$idir" "$scripts_dir"
	echo "$out_dir/$out_name"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	apk_build "$1" "$2"
fi
