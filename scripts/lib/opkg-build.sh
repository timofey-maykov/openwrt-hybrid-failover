#!/usr/bin/env bash
# Build OpenWrt 24.x compatible .ipk (gzip-wrapped tar, not ar).
set -euo pipefail

opkg_build() {
	local pkg_root="$1"
	local out_dir="$2"
	local control="$pkg_root/CONTROL/control"

	[[ -f "$control" ]] || { echo "Missing CONTROL/control in $pkg_root" >&2; return 1; }
	command -v tar >/dev/null 2>&1 || { echo "tar not found" >&2; return 1; }

	local pkg_name pkg_version pkg_arch
	pkg_name="$(awk -F': ' '/^Package:/{print $2; exit}' "$control")"
	pkg_version="$(awk -F': ' '/^Version:/{print $2; exit}' "$control")"
	pkg_arch="$(awk -F': ' '/^Architecture:/{print $2; exit}' "$control")"

	[[ -n "$pkg_name" && -n "$pkg_version" && -n "$pkg_arch" ]] || {
		echo "Invalid control file: $control" >&2
		return 1
	}

	mkdir -p "$out_dir"
	local work ipk_name
	work="$(mktemp -d)"
	ipk_name="${pkg_name}_${pkg_version}_${pkg_arch}.ipk"

	mkdir -p "$work/CONTROL"
	cp "$control" "$work/CONTROL/"
	if [[ -f "$pkg_root/CONTROL/postinst" ]]; then
		cp "$pkg_root/CONTROL/postinst" "$work/CONTROL/"
		chmod 755 "$work/CONTROL/postinst"
	fi
	if [[ -f "$pkg_root/CONTROL/prerm" ]]; then
		cp "$pkg_root/CONTROL/prerm" "$work/CONTROL/"
		chmod 755 "$work/CONTROL/prerm"
	fi
	[[ -f "$pkg_root/CONTROL/conffiles" ]] && cp "$pkg_root/CONTROL/conffiles" "$work/CONTROL/"

	(
		cd "$work/CONTROL"
		tar --numeric-owner --owner=0 --group=0 -czf "$work/control.tar.gz" .
	)
	(
		cd "$pkg_root"
		tar --numeric-owner --owner=0 --group=0 -czf "$work/data.tar.gz" \
			--exclude=CONTROL .
	)

	printf '2.0\n' >"$work/debian-binary"
	rm -f "$out_dir/$ipk_name"
	(
		cd "$work"
		tar --numeric-owner --owner=0 --group=0 -czf "$out_dir/$ipk_name" \
			debian-binary control.tar.gz data.tar.gz
	)

	rm -rf "$work"
	echo "$out_dir/$ipk_name"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	opkg_build "$1" "$2"
fi
