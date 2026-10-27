[Русский](README.md)

# LuCI (`luci-app-hybrid-failover`)

The Hybrid Failover web interface for OpenWrt.

**User guide (tabs, buttons, when changes take effect):** [docs/en/LUCI.md](../docs/en/LUCI.md)

---

## Menu and paths

**Services → Hybrid Failover**, base URL `/cgi-bin/luci/admin/services/hybrid-failover`. The menu root opens `/dashboard`.

| Path | View |
|------|------|
| `/dashboard` | `dashboard.js` |
| `/routing` | `routing.js` |
| `/services` | `services.js` |
| `/tunnels` | `tunnels.js` |
| `/diagnostics` | `diagnostics.js` |
| `/clients` | `clients.js` |
| `/bot` | `bot.js` |
| `/update` | `update.js` |

What each tab does is described in [docs/en/LUCI.md](../docs/en/LUCI.md#tabs). Menu titles are set in `menu.d` in Russian and translated by the catalog like every other string. Every entry requires the `luci-app-hybrid-failover` ACL. The `/bot` entry shows up only when `/usr/bin/hybrid-failover-bot` exists and is executable.

---

## Sources

| Path | Contents |
|------|----------|
| `luci/root/www/luci-static/resources/view/hybrid-failover/` | LuCI pages (JS), one per tab |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js` | Shared UI: page header, cards, tables, modals, overall status, RPC wrappers |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-charts.js` | Charts for `tunnels.js`: canvas, legend, shared crosshair, zoom. No external libraries |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json` | Menu |
| `luci/root/usr/share/rpcd/ucode/hybrid-failover` | rpcd backend (ucode) |
| `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json` | ACL |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json`, `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json` | Menu and ACL of the old standalone `luci-app-hybrid-failover-bot` package (only an SDK Makefile in `packages/`, not part of releases) |
| `luci/po/en/hybrid-failover.po`, `luci/po/ru/hybrid-failover.po`, `luci/po/zh-cn/hybrid-failover.po` | Translation catalogs |
| `luci/i18n/hybrid-failover.en.lmo`, `luci/i18n/hybrid-failover.zh-cn.lmo` | Compiled English and Chinese catalogs |

### Translations

The source strings in the JS are Russian, so the Russian UI works without a catalog. `po/ru` is nearly empty and exists for tooling only. The English and Simplified Chinese catalogs cover almost every interface string. A string with no translation shows in Russian. Add new `_('…')` strings to both catalogs.

After editing a `.po`, rebuild the catalogs by hand:

```sh
./scripts/compile-luci-i18n.sh
```

The script needs `po2lmo`. If it is not in `PATH`, the script clones `openwrt/luci` into `.cache/luci-po2lmo` once and builds it.

---

## Backend

The pages call the `hybrid-failover` ubus object, which the rpcd ucode script registers. The script runs `/usr/sbin/hybrid-failover` and passes its JSON back.

```text
LuCI (browser) → ubus hybrid-failover.<method> → rpcd ucode → hybrid-failover rpc <Method>
```

The reply is `{ ok, data }` when the core printed JSON, otherwise `{ ok, output }` or `{ ok, code, output }`. `ok` mirrors the core exit code, so a failed check returns both `ok: false` and the parsed JSON.

### ubus methods

| ubus | Arguments | Core command | Called by |
|------|-----------|--------------|-----------|
| `status` | | `rpc Status` | Overview, Routing, Clients |
| `health` | | `rpc Health` | Overview ("Check channels now"), 60 s timeout |
| `history` | | `rpc History` | Overview |
| `export_history` | | `rpc ExportHistory` | Overview ("Export history") |
| `delay_history` | | `rpc DelayHistory` | Overview |
| `metrics` | | `rpc Metrics` | Declared in `hf-ui.js`, not called by the pages |
| `list_routes` | | `rpc ListRoutes` | Overview, Routing, Services and channels |
| `validate` | | `rpc Validate` | Routing, Diagnostics |
| `check_nft` | | `rpc CheckNft` | Diagnostics |
| `check_fakeip` | | `rpc CheckFakeip` | Overview, Diagnostics |
| `global_check` | | `rpc GlobalCheck` | Overview, Diagnostics, 60 s timeout |
| `switch_proxy` | `section`, `outbound` | `rpc SwitchProxy <section> <outbound>` | Overview |
| `decode_uri` | `uri` | `rpc DecodeURI <uri>` | Routing ("Decode") |
| `apply` | | `hybrid-failover apply` | Routing (when the form has no changes) |
| `reload` | | `hybrid-failover reload` | Not called by the pages |
| `list_update` | | `rpc ListUpdate` | Routing |
| `subscription_refresh` | | `rpc SubscriptionRefresh` | Routing |
| `subscription_status` | | `rpc SubscriptionStatus` | Routing (Subscriptions tab) |
| `duplicate_section` | `from`, `to` | `rpc DuplicateSection <from> <to>` | Routing ("Copy section…") |
| `list_clients` | | `rpc ListClients` | Clients |
| `dhcp_leases` | | No core call, see below | Clients |
| `backup_uci` | | `rpc BackupUCI` | Diagnostics |
| `backup_download` | | `rpc BackupDownload` | Diagnostics |
| `restore_uci` | `path` | `rpc RestoreUCI <path>` | Diagnostics |
| `pending_capture`, `pending_validate`, `pending_apply`, `pending_rollback` | | `rpc CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` | Not called by the pages |
| `update_check` | | `rpc UpdateCheck` | Update, 100 s timeout |
| `update_status` | | `rpc UpdateStatus` | Update |
| `update_apply` | | `rpc UpdateApply` | Update |

For `restore_uci` and `duplicate_section`, empty arguments are not passed to the core. The ACL allows all of these methods. `write` holds `apply`, `reload`, `pending_capture`, `pending_apply`, `pending_rollback`, `switch_proxy`, `restore_uci`, `list_update`, `subscription_refresh`, `update_check`, `update_apply`; the rest are in `read`. `duplicate_section` is in `read` too, even though it writes UCI.

`dhcp_leases` reads the dnsmasq lease files itself (paths from `leasefile` in UCI `dhcp`, otherwise `/tmp/dhcp.leases`) and falls back to `ubus call dhcp ipv4leases` if they are empty. Calling `luci-rpc` from inside rpcd deadlocked, which is why it is not used.

### What bypasses the ubus object

- **Saving forms.** Routing, Services and channels, and Clients write UCI through the standard LuCI `uci` and apply with `ui.changes.apply(true)`, i.e. with rollback on lost connectivity. After the commit procd runs `hybrid-failover reload` through `procd_add_reload_trigger`. The Routing "Save and apply" button with no form changes calls `validate`, then `apply`.
- **Charts.** `tunnels.js` reads `/var/run/hybrid-failover/channel-metrics.json` and `channel-metrics-24h.json` through `fs.read_direct`.
- **Telegram.** `bot.js` reads `/etc/hybrid-failover-bot.json` through `fs.read` and calls `/usr/bin/hybrid-failover-bot -mode set-pending`, then `-mode apply-config`, through `fs.exec`. On error it calls `-mode rollback-config`. The bot is restarted through ubus `rc init` (`restart`), and its state is read from `service list`. `rc init` is used because `service restart` does not bring up a bot that was off at boot.

The main ACL allows all of that. Latency history lives in `/var/run/hybrid-failover/delay-history.json`, the switch log in `/var/log/hybrid-failover/history.jsonl`. The native engine has no Clash-style HTTP API; status and channels come only through RPC.

The UCI configs are `/etc/config/hybrid-failover` and `/etc/config/hybrid-failover-bot`.

rpcd loads `/usr/share/rpcd/ucode/*` only at startup. After editing the ucode script, run `/etc/init.d/rpcd restart`.

---

## Build and install

```sh
./scripts/build-packages.sh
```

Packages land in `dist/ipk` (opkg, OpenWrt 24.x) and `dist/apk` (apk, 25.12+). `HF_PKG_FORMAT=ipk|apk|both` picks the format, `both` by default. `HF_BUILD_SET=core` builds only the core and the bot, without LuCI.

Both LuCI packages, `luci-app-hybrid-failover` and `luci-i18n-hybrid-failover`, are the same for every architecture. The easiest way to install them is the one-line command on the router, see [docs/en/INSTALL.md](../docs/en/INSTALL.md).

`luci-app-hybrid-failover` depends on `luci-base`, `luci-compat`, `luci-i18n-hybrid-failover` and `hybrid-failover-core`.

The package postinst sets `luci.main.rpctimeout` to 60 seconds when it is unset or between 10 and 59 (the channel check and the full check can take longer than the default 20 seconds), restarts rpcd and uhttpd and clears the LuCI cache.

---

## Checking on the router

```sh
ubus call hybrid-failover status '{}'
hybrid-failover rpc Status
```

`scripts/luci-ubus-smoke.sh` calls `status`, `health`, `history`, `check_nft`, `global_check`, `export_history`, `delay_history`, `list_clients`, `dhcp_leases` and checks that each one answers. Then it calls `switch_proxy` for section `glob` (except under the `fastest` policy), so it really switches the channel. Run it on a test router.
