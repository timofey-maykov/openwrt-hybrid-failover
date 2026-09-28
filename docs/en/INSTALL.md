[Русский](../INSTALL.md)

# Installing on OpenWrt

## One command on the router

Packages come from the [GitHub releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases). Every release carries `.ipk` files for OpenWrt 24.x and `.apk` files for 25.12+.

```sh
# OpenWrt 24.x (opkg):
opkg update && opkg install curl ca-bundle wget
# OpenWrt 25.12+ (apk):
# apk update && apk add curl ca-bundle wget

wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh

ash /tmp/install.sh
```

The script picks the package manager (`apk` or `opkg`) and reads the architecture from `/etc/openwrt_release`, then downloads and installs the matching files. Afterwards it clears the LuCI cache and restarts `rpcd` and `uhttpd`. In `full` and `core` modes it also runs `hybrid-failover migrate`, enables and starts the `hybrid-failover` service and checks the config with `hybrid-failover validate`. There is no need to run `migrate` or `start` yourself after the script.

No external sing-box package is needed. The engine is built into `hybrid-failover-core`.

### Install modes

| `HF_MODE` | What gets installed |
|-----------|---------------------|
| `full` (default) | `hybrid-failover-core`, `luci-i18n-hybrid-failover`, `luci-app-hybrid-failover`, `hybrid-failover-bot` |
| `core` | the same without the bot |
| `bot` | `hybrid-failover-bot` only |

The mode is set with an environment variable, for example.

```sh
HF_MODE=core ash /tmp/install.sh
```

The bot package depends on `hybrid-failover-core`, so `bot` mode is meant for a router that already has the core. There is no LuCI page for the bot in this mode, since the separate package for it is gone and `luci-app-hybrid-failover` requires the core. In `bot` mode the script leaves the `hybrid-failover` service alone. In `core` mode the LuCI **Telegram** tab stays hidden until `/usr/bin/hybrid-failover-bot` is installed.

### Environment variables

```sh
HF_REPO=timofey-maykov/openwrt-hybrid-failover   # GitHub repository
HF_VERSION=latest            # or a specific tag such as v1.7.48
HF_BRANCH=main               # branch to read VERSION from if the GitHub API is unreachable
HF_TOKEN=123456789:ABC...    # bot token, written straight into /etc/hybrid-failover-bot.json
HF_ADMIN_IDS=123456789       # only printed as a hint, admin_ids are not written to the JSON
HF_LOW_SPACE_KB=45000        # free space threshold on /overlay, in KB
HF_FORCE_REINSTALL=1         # remove the old package before installing the new one
HF_DIST_DIR=/tmp/hf-dist     # directory with local packages (ipk/ and apk/)
```

If `HF_DIST_DIR` has an `ipk/` or `apk/` subdirectory for the current package manager, the script installs from there and does not contact GitHub.

### After installing

1. If the bot is installed, edit `/etc/hybrid-failover-bot.json` and set at least `token` and `admin_ids`. The LuCI **Telegram** tab does the same. The other keys are described in [bot/README.en.md](../../bot/README.en.md).
2. Enable the bot. It is off after installation:
   ```sh
   uci set hybrid-failover-bot.main.enabled=1
   uci commit hybrid-failover-bot
   /etc/init.d/hybrid-failover-bot restart
   ```
3. Open LuCI at **Services → Hybrid Failover**
   (`http://ROUTER/cgi-bin/luci/admin/services/hybrid-failover`).
   The usual order is **Overview** for status, **Routing** for sections and URIs, **Clients** for per-IP rules. See [LUCI.md](LUCI.md).
4. Open your bot in Telegram and send `/panel`.

## Installing packages by hand

Download the packages from a release or build them on a PC:

```sh
./scripts/build-packages.sh
```

By default this builds both `.ipk` and `.apk` into `dist/ipk/` and `dist/apk/`. Build details are in [packages/README.en.md](../../packages/README.en.md).

To see the router architecture:

```sh
. /etc/openwrt_release && echo "$DISTRIB_ARCH"
```

Releases are built for `aarch64_cortex-a53`, `aarch64_generic`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc` and `x86_64`. The LuCI packages are architecture independent.

### OpenWrt 24.x (opkg, `.ipk`)

```sh
opkg install /tmp/hybrid-failover-core_1.7.48-1_aarch64_cortex-a53.ipk
opkg install /tmp/hybrid-failover-bot_1.7.48-1_aarch64_cortex-a53.ipk
opkg install /tmp/luci-i18n-hybrid-failover_1.7.48-1_all.ipk
opkg install /tmp/luci-app-hybrid-failover_1.7.48-1_all.ipk
```

### OpenWrt 25.12+ (apk, `.apk`)

Since 25.12 OpenWrt uses apk (Alpine Package Keeper) instead of opkg. File names follow `package-version-rN_arch.apk`, and architecture independent packages have no arch suffix.

```sh
apk add --allow-untrusted /tmp/hybrid-failover-core-1.7.48-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/hybrid-failover-bot-1.7.48-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/luci-i18n-hybrid-failover-1.7.48-r1.apk
apk add --allow-untrusted /tmp/luci-app-hybrid-failover-1.7.48-r1.apk
```

The packages are not signed with the OpenWrt key, hence `--allow-untrusted`.

### First start after a manual install

The core package's `postinst` creates `/etc/hybrid-failover/pending` and runs `migrate`. Enable and start the service yourself:

```sh
/etc/init.d/hybrid-failover enable
/etc/init.d/hybrid-failover start
```

`hybrid-failover migrate` brings UCI up to the current schema. It imports the legacy config if there is no config of its own yet, disables the legacy init script, switches `engine_mode` to `native` and warns about conflicting scripts. An external sing-box gets stopped and disabled, and on opkg its package is removed as well. The command is safe to run again and prints `migration: no changes` when there is nothing to do. With `--dry-run` it only shows the plan.

## Installing from a PC over SSH

```sh
./scripts/install-from-local-dist.sh 192.168.42.1   # copies dist/ipk and dist/apk to /tmp/hf-dist and runs the installer
./scripts/deploy-telegram-bot.sh 192.168.42.1       # bot binary and its config files only, for development
```

The SSH user is `root` unless `ROUTER_USER` says otherwise. `deploy-telegram-bot.sh` builds the bot for `mipsle` (softfloat), so for other architectures build the binary yourself. The script also copies `/etc/hybrid-failover-bot.json` from the repository, which overwrites the config on the router.

## Dependencies

| Package | Depends |
|---------|---------|
| `hybrid-failover-core` | `ca-bundle`, `uci`, `procd` |
| `hybrid-failover-bot` | `libc`, `procd`, `ca-bundle`, `uci`, `hybrid-failover-core` |
| `luci-app-hybrid-failover` | `luci-base`, `luci-compat`, `luci-i18n-hybrid-failover`, `hybrid-failover-core` |
| `luci-i18n-hybrid-failover` | `luci-base` |

The installer also installs `curl`, `ca-bundle` and `wget` from the OpenWrt feeds. The package manager pulls in the other dependencies from the feeds on its own.

## AmneziaWG 3.1

`kmod-amneziawg` and `amneziawg-tools` are not part of the releases and have to be installed separately. You need them when urltest or failover contains `vpn://` or `awg2://` links with Amnezia 3.1 parameters (`RandomTrailers`, `HeaderProtectionKey`).

| Package | Requirement |
|---------|-------------|
| `kmod-amneziawg` | 3.1 or newer, vermagic matches the router kernel |
| `amneziawg-tools` | 3.1 or newer, `awg set --help` mentions `random-trailers` |

The build must match `DISTRIB_RELEASE` and the kernel version (`opkg info kernel` or `apk info kernel`). Ready packages are available in the [2Grey/awg-openwrt](https://github.com/2Grey/awg-openwrt) releases, tagged by OpenWrt version such as `v24.10.6`. Pick the files for your target and architecture, for example `ramips/mt7621` and `mipsel_24kc`.

`amneziawg-tools` depends on the virtual package `ip`. On an image where `ip` comes from busybox, install `ip-tiny` or `ip-full`, or install the tools with `--force-depends`.

The handshake and URI fields are described in [OVERVIEW.md](OVERVIEW.md).

## Releases and updates

A `v*` tag triggers the [build workflow](../../.github/workflows/release.yml). It builds `.ipk` and `.apk` for every architecture and publishes them to [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) together with `manifest.json`, which records the sha256 and size of each file. Pushes to `main` only check that the packages build.

An installed system can update itself from the router:

```sh
hybrid-failover update check     # latest release on GitHub and whether it is newer
hybrid-failover update apply     # install the latest release in the background
hybrid-failover update status    # progress and log of the last update
```

`apply` downloads `hybrid-failover-core`, `luci-i18n-hybrid-failover` and `luci-app-hybrid-failover`, plus `hybrid-failover-bot`, `curfew` and `luci-app-curfew` if they are already installed. Every file is checked against `manifest.json`. After installing it runs `migrate` and restarts `rpcd`, `hybrid-failover` and the bot. The log goes to `/tmp/hybrid-failover/update/update.log`. `--foreground` runs the update in the current console, `--tag v1.7.48` installs a specific release, `--force` allows the same or an older version. The `HF_REPO` variable points it at another repository.

In LuCI the same thing lives on the **Update** tab, see [LUCI.md](LUCI.md).

## Low space on overlay

Release binaries are compressed with UPX. When `/overlay` has less free space than `HF_LOW_SPACE_KB` (45000 KB by default), the installer stops the services, removes the installed package and puts the new one in its place. The same behaviour can be forced:

```sh
HF_FORCE_REINSTALL=1 ash /tmp/install.sh
```
