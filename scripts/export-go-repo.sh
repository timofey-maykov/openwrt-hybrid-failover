#!/usr/bin/env bash
# Export Go core + bot into a standalone git tree (опционально, вне GitHub).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT_DIR/../openwrt-hybrid-failover-core-export}"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

copy() {
	local src="$1" dst="$2"
	[[ -e "$ROOT_DIR/$src" ]] || return 0
	mkdir -p "$(dirname "$OUT_DIR/$dst")"
	cp -R "$ROOT_DIR/$src" "$OUT_DIR/$dst"
}

# Go modules
copy go.mod go.mod
copy go.sum go.sum 2>/dev/null || true
copy core core
copy internal internal
copy bot bot

# OpenWrt integration
copy openwrt openwrt

# Core package + examples + CI
copy packages/hybrid-failover-core packages/hybrid-failover-core
copy packages/hybrid-failover-bot packages/hybrid-failover-bot
copy examples/testdata examples/testdata
copy examples/glob-uci-commands.txt examples/glob-uci-commands.txt
copy .github/workflows/go-ci.yml .github/workflows/go-ci.yml
mkdir -p "$OUT_DIR/.github/workflows"
cp "$ROOT_DIR/packaging/core-release/release.yml" "$OUT_DIR/.github/workflows/release.yml"
copy scripts/verify-smoke.sh scripts/verify-smoke.sh
copy scripts/verify-qemu.sh scripts/verify-qemu.sh
copy scripts/lib scripts/lib
copy scripts/build-packages.sh scripts/build-packages.sh
copy VERSION VERSION

# Docs (core-focused)
mkdir -p "$OUT_DIR/docs"
for f in UCI.md OVERVIEW.md INSTALL.md INSTALL.md; do
	copy "docs/$f" "docs/$f"
done
# docs: product guides only

# README for core repo
cat >"$OUT_DIR/README.md" <<'EOF'
# openwrt-hybrid-failover-core

Go implementation of **Hybrid Failover** for OpenWrt: `/usr/sbin/hybrid-failover`, UCI → sing-box, nft tproxy, failover controller, Telegram bot.

> Full distribution (LuCI, install scripts, all packages): [openwrt-hybrid-failover](https://github.com/timofey-maykov/openwrt-hybrid-failover)

## Build

```sh
go build -o hybrid-failover ./core/cmd/hybrid-failover
cd bot && go build -o hybrid-failover-bot ./cmd/hybrid-failover-bot
```

## Test

```sh
go test ./...
./scripts/verify-smoke.sh
cd bot && go test ./...
```

## OpenWrt packages

Сборка **hybrid-failover-core** и **hybrid-failover-bot** (`.ipk` / `.apk`):

```sh
HF_BUILD_SET=core ./scripts/build-packages.sh
```

Готовые пакеты: [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) (тег `v*`).

See [docs/OVERVIEW.md](docs/OVERVIEW.md) and [docs/UCI.md](docs/UCI.md).
EOF

echo "Exported to $OUT_DIR"
