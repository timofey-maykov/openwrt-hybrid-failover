#!/usr/bin/env bash
# Native engine smoke: compile, unit tests, validate all fixtures.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTDATA_DIR="${ROOT_DIR}/examples/testdata"
BIN="${HF_BIN:-}"

die() { printf 'verify-native: ERROR: %s\n' "$*" >&2; exit 1; }
ok() { printf 'verify-native: OK: %s\n' "$*"; }

if [[ -z "$BIN" ]]; then
	if command -v hybrid-failover >/dev/null 2>&1; then
		BIN="$(command -v hybrid-failover)"
	else
		BIN="$(mktemp "${TMPDIR:-/tmp}/hybrid-failover-native.XXXXXX")"
		trap 'rm -f "$BIN"' EXIT
		( cd "$ROOT_DIR" && go build -mod=mod -trimpath -o "$BIN" ./core/cmd/hybrid-failover )
	fi
fi
[[ -x "$BIN" ]] || die "binary missing: $BIN"

(
	cd "$ROOT_DIR"
	GOFLAGS=-mod=mod go test ./internal/engine/... ./internal/failover/... ./internal/lifecycle/...
)
ok "go test engine/failover/lifecycle"

(
	cd "$ROOT_DIR"
	GOFLAGS=-mod=mod go test ./internal/engine/plan/ -run TestCompilePlanFromExamples
)
ok "CompilePlan all testdata"

[[ -d "$TESTDATA_DIR" ]] || die "missing $TESTDATA_DIR"
count=0
for fixture in "$TESTDATA_DIR"/*.conf; do
	[[ -f "$fixture" ]] || continue
	count=$((count + 1))
	name="$(basename "$fixture")"
	if ! "$BIN" validate --engine native --dry-run --uci "$fixture" >/dev/null 2>&1; then
		"$BIN" validate --engine native --dry-run --uci "$fixture" 2>&1 || true
		die "validate --engine native failed: $name"
	fi
	ok "validate native: $name"
done
[[ "$count" -gt 0 ]] || die "no fixtures"

printf 'verify-native: all checks passed (%d fixtures)\n' "$count"
