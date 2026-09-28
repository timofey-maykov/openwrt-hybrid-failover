[Русский](README.md)

# OpenWrt packages

The packages are built on a PC or in CI without the OpenWrt SDK. You need Go and tar, plus apk-tools 3.x or Docker for `.apk`.

```sh
chmod +x scripts/build-packages.sh scripts/lib/*.sh
./scripts/build-packages.sh
```

The script cross-compiles `hybrid-failover` (core) and `hybrid-failover-bot` for each architecture, lays the files out into packages and writes `dist/manifest.json` with the sha256 and size of every package. `hybrid-failover update` relies on that manifest later.

## Build variables

| Variable | Default | Effect |
|----------|---------|--------|
| `HF_PKG_FORMAT` | `both` | `ipk`, `apk` or `both` |
| `HF_UPX` | `auto` | `auto` compresses binaries with UPX when `upx` is in PATH. `1` requires UPX, `0` turns it off |
| `HF_BUILD_SET` | `full` | `full` builds everything, `core` builds only the core and the bot, without LuCI |
| `HF_BUILD_ARCHS_OVERRIDE` | | space separated architectures, for example `"x86_64"` |
| `PKG_RELEASE` | `1` | package revision (`-1` in ipk, `-r1` in apk) |
| `DIST_DIR` | `dist` | output directory |
| `APK_DOCKER_IMAGE` | `alpine:edge` | image used for `apk mkpkg` when the host has no suitable `apk` |

The version comes from the `VERSION` file at the repository root. CI (`.github/workflows/release.yml`) builds with `HF_UPX=1`, and UPX makes the binaries about 60% smaller.

## Output

| Directory | Format | OpenWrt |
|-----------|--------|---------|
| `dist/ipk/` | `.ipk` (opkg) | 24.10 and older |
| `dist/apk/` | `.apk` (apk-tools 3) | 25.12+ |
| `dist/binaries/<arch>/` | bare binaries | |

Supported architectures are `aarch64_cortex-a53`, `aarch64_generic`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc` and `x86_64`.

### File names

| opkg (24.x) | apk (25.12+) |
|-------------|--------------|
| `hybrid-failover-core_1.7.48-1_aarch64_cortex-a53.ipk` | `hybrid-failover-core-1.7.48-r1_aarch64_cortex-a53.apk` |
| `luci-app-hybrid-failover_1.7.48-1_all.ipk` | `luci-app-hybrid-failover-1.7.48-r1.apk` |

The apk version follows the [OpenWrt scheme](https://git.openwrt.org/?p=openwrt/openwrt.git;a=commit;h=e8725a932e16eaf6ec51add8c084d959cbe32ff2), with the revision written as `-rN`. Architecture independent packages (`all` in ipk, `noarch` in apk) have no arch suffix in the apk file name.

## Contents

| Package | Architecture | Contents |
|---------|--------------|----------|
| `hybrid-failover-core` | per target | `/usr/sbin/hybrid-failover` with the built-in engine, init.d, `/etc/config/hybrid-failover`, `/etc/sysctl.d/20-hybrid-failover.conf` |
| `hybrid-failover-bot` | per target | `/usr/bin/hybrid-failover-bot`, init.d, UCI `hybrid-failover-bot`, `/etc/hybrid-failover-bot.json` |
| `luci-app-hybrid-failover` | all | LuCI pages for overview, routing, clients, diagnostics, Telegram and update |
| `luci-i18n-hybrid-failover` | all | LuCI translations |
| `curfew` | all | night bandwidth limits for selected LAN devices (tc) |
| `luci-app-curfew` | all | LuCI for curfew |

No external sing-box is needed, the engine is built into the core. `kmod-amneziawg` and `amneziawg-tools` are not included. For AmneziaWG 3.1 see [docs/en/INSTALL.md](../docs/en/INSTALL.md#amneziawg-31).

`/etc/config/hybrid-failover`, `/etc/config/hybrid-failover-bot` and `/etc/hybrid-failover-bot.json` are declared as conffiles and survive package upgrades. The core postinst creates `/etc/hybrid-failover/pending`, runs `hybrid-failover migrate` and applies the sysctl settings. The bot postinst enables its init script, but the bot only starts with `hybrid-failover-bot.main.enabled=1`.

## Installing on the router

`scripts/install-on-router.sh` picks `apk` or `opkg` and downloads the matching files from GitHub Releases.

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

By default (`HF_MODE=full`) it installs `hybrid-failover-core`, `luci-i18n-hybrid-failover`, `luci-app-hybrid-failover` and `hybrid-failover-bot`, with the translations going in before the LuCI app that depends on them. `HF_MODE=core` installs the same without the bot, and `HF_MODE=bot` installs only the bot on a router that already has the core.

Freshly built packages can be installed from a PC with `./scripts/install-from-local-dist.sh <router ip>`. More in [docs/en/INSTALL.md](../docs/en/INSTALL.md).

## OpenWrt SDK

The Makefiles in `hybrid-failover-core/`, `hybrid-failover-bot/` and `luci-app-hybrid-failover-bot/` are kept for building inside an OpenWrt tree, but they lag behind `build-packages.sh`. The version is hardcoded as `1.0.0`, the core Makefile installs only the binary and the init script, and `luci-app-hybrid-failover-bot` describes the old standalone bot page that releases no longer ship. The binaries are built outside the SDK either way. Run `scripts/build-packages.sh` first, then put the bot binary into `packages/hybrid-failover-bot/binaries/<ARCH>/`. The core Makefile reads its binary from `dist/binaries/<ARCH>/`.
