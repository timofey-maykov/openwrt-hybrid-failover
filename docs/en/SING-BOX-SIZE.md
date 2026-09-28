[Русский](../SING-BOX-SIZE.md)

# Package size and overlay

Hybrid Failover used to run on top of the sing-box package, and routers with a 64-128 MiB overlay filled up quickly. Stock sing-box took about 40 MiB. Routing is now done by the native engine inside `hybrid-failover`, and sing-box is no longer needed.

## sing-box is no longer installed

The `engine_mode=singbox` mode was removed. `hybrid-failover migrate` switches old installs to `native`, stops and disables `/etc/init.d/sing-box`, removes the `sing-box` package through opkg and deletes `/etc/sing-box/config.json`. On apk systems remove a leftover sing-box package by hand (`apk del sing-box`).

The script `scripts/build-sing-box-lite.sh` is still in the repository. It built a trimmed sing-box without tailscale, wireguard and dhcp. Current versions do not need it.

## What takes space now

| Package | Contents | Needed |
|---------|----------|--------|
| `hybrid-failover-core` | `/usr/sbin/hybrid-failover` with the engine, DNS on `127.0.0.42` and the failover controller | always |
| `luci-app-hybrid-failover`, `luci-i18n-hybrid-failover` | LuCI pages and translations | if you want the web UI |
| `hybrid-failover-bot` | `/usr/bin/hybrid-failover-bot` | only for Telegram |

Community lists live in `/etc/hybrid-failover/rulesets/` and also use overlay space. How much depends on the selected `community_lists`.

`bind-dig` and `bind-libs` are not needed. `hybrid-failover check-fakeip` checks DNS with its own Go client, so these packages can be removed (`opkg remove bind-dig bind-libs`).

## UPX compression

After building, `scripts/build-packages.sh` runs core and the bot through UPX. The `HF_UPX` variable controls this.

| Value | What happens |
|-------|--------------|
| `auto` (default) | compress if `upx` is in `PATH`, skip otherwise |
| `1` | compression is required, the build fails without `upx` |
| `0` | do not compress |

The release build in GitHub Actions installs `upx-ucl` and builds with `HF_UPX=1`. The script prints each binary's size before and after compression. A compressed binary starts slightly slower on the router, by a fraction of a second.

## Installing with little free space

`scripts/install-on-router.sh` checks free space on `/overlay`. If it is below the threshold or `HF_FORCE_REINSTALL=1` is set, the installer stops the services and removes the installed package before installing the new one. With opkg it installs packages with `--force-space`.

| Variable | Meaning |
|----------|---------|
| `HF_LOW_SPACE_KB` | threshold in KB, 45000 by default |
| `HF_FORCE_REINSTALL` | `1` removes and reinstalls our packages regardless of free space |

Old installs left a monolithic `/usr/bin/hybrid-failover` binary of more than 6 MiB. The installer deletes it if `/usr/sbin/hybrid-failover` is already present.

## Recommendations

1. On a router with a 64 MiB overlay install only `hybrid-failover-core` and LuCI.
2. Without Telegram you do not need `hybrid-failover-bot`.
3. If sing-box is still around after upgrading from an old version, check that `migrate` removed it (`opkg list-installed | grep sing-box`).
4. If space is still tight, extroot helps.
