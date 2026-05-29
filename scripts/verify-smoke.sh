#!/usr/bin/env bash
# Basic regression smoke checks for hybrid-failover-core (host or router).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTDATA_DIR="${ROOT_DIR}/examples/testdata"
TMP_UCI=""
BIN=""

cleanup() {
	[[ -n "$TMP_UCI" && -f "$TMP_UCI" ]] && rm -f "$TMP_UCI"
	if [[ "${HF_SMOKE_BUILT:-0}" = "1" && -n "$BIN" && -f "$BIN" ]]; then
		rm -f "$BIN"
	fi
}
trap cleanup EXIT

die() { printf 'verify-smoke: ERROR: %s\n' "$*" >&2; exit 1; }
ok() { printf 'verify-smoke: OK: %s\n' "$*"; }

if [[ -n "${HF_BIN:-}" ]]; then
	BIN="$HF_BIN"
elif command -v hybrid-failover >/dev/null 2>&1; then
	BIN="$(command -v hybrid-failover)"
else
	BIN="$(mktemp "${TMPDIR:-/tmp}/hybrid-failover-smoke.XXXXXX")"
	HF_SMOKE_BUILT=1
	(
		cd "$ROOT_DIR"
		go build -mod=mod -trimpath -o "$BIN" ./core/cmd/hybrid-failover
	)
fi

[[ -x "$BIN" ]] || die "binary not found or not executable: $BIN"
ok "binary exists: $BIN"

if ! "$BIN" help >/dev/null 2>&1; then
	"$BIN" help 2>&1 || true
	die "help command failed"
fi
ok "help output"

[[ -d "$TESTDATA_DIR" ]] || die "missing testdata dir: $TESTDATA_DIR"

fixture_needs_cli_validate() {
	# Matches internal/uci HasOutboundSection: proxy/vpn sections with outbounds.
	grep -qE "option interface |option proxy_string |option outbound_json |failover_proxy_links|urltest_proxy_links" "$1"
}

fixture_count=0
for fixture in "$TESTDATA_DIR"/*.conf; do
	[[ -f "$fixture" ]] || continue
	fixture_count=$((fixture_count + 1))
	name="$(basename "$fixture")"
	if fixture_needs_cli_validate "$fixture"; then
		TMP_UCI="$(mktemp "${TMPDIR:-/tmp}/hybrid-failover-uci.XXXXXX")"
		cp "$fixture" "$TMP_UCI"
		if ! "$BIN" validate --dry-run --uci "$TMP_UCI" >/dev/null 2>&1; then
			"$BIN" validate --dry-run --uci "$TMP_UCI" 2>&1 || true
			die "validate on fixture $name failed"
		fi
		rm -f "$TMP_UCI"
		TMP_UCI=""
		ok "validate fixture: $name"
	else
		ok "fixture $name (builder-only; covered by go test)"
	fi
done

[[ "$fixture_count" -gt 0 ]] || die "no *.conf fixtures in $TESTDATA_DIR"
ok "validated $fixture_count fixture(s)"

(
	cd "$ROOT_DIR"
	GOFLAGS=-mod=mod go test ./internal/singbox/...
)
ok "go test ./internal/singbox/..."

if command -v sing-box >/dev/null 2>&1; then
	(
		cd "$ROOT_DIR"
		GOFLAGS=-mod=mod go test -run TestBuilderSingboxCheckOptional ./internal/singbox/...
	)
	ok "sing-box check on generated configs"
else
	printf 'verify-smoke: SKIP: sing-box not installed (optional config check)\n'
fi

printf 'verify-smoke: all checks passed\n'
