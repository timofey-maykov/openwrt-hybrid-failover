[Русский](README.md)

# LuCI (`luci-app-hybrid-failover`)

The Hybrid Failover web interface for OpenWrt.

**User guide (what to click and why):** [docs/en/LUCI.md](../docs/en/LUCI.md)

---

## Menu and paths

**Сервисы → Hybrid Failover** (Services → Hybrid Failover), base URL `/cgi-bin/luci/admin/services/hybrid-failover`. The menu root opens the Overview tab.

Tab titles come from the menu JSON and are in Russian.

| Tab | Path | View | Purpose |
|-----|------|------|---------|
| Обзор (Overview) | `/dashboard` | `dashboard.js` | Status, channels, failover controller, delay history, manual switch |
| Маршрутизация (Routing) | `/routing` | `routing.js` | Global settings, sections, URIs, validate and apply |
| Диагностика (Diagnostics) | `/diagnostics` | `diagnostics.js` | Validate, check-nft, check-fakeip, global-check, UCI backup |
| Клиенты (Clients) | `/clients` | `clients.js` | `client_rule`, pick from DHCP, effective rules |
| Telegram | `/bot` | `bot.js` | Bot service, JSON config through pending |
| Обновление (Update) | `/update` | `update.js` | Check GitHub releases and install |

---

## Sources

| Path | Contents |
|------|----------|
| `luci/root/www/luci-static/resources/view/hybrid-failover/` | LuCI pages (JS) |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js` | Shared UI: cards, tables, modals, RPC wrappers |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json` | Menu |
| `luci/root/usr/share/rpcd/ucode/hybrid-failover` | rpcd backend (ucode) |
| `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json` | ACL |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json`, `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json` | Menu and ACL of the old standalone `luci-app-hybrid-failover-bot` package (only an SDK Makefile in `packages/`, not part of releases) |
| `luci/po/en/hybrid-failover.po`, `luci/po/ru/hybrid-failover.po`, `luci/po/zh-cn/hybrid-failover.po` | Translation catalogs |
| `luci/i18n/hybrid-failover.en.lmo`, `luci/i18n/hybrid-failover.zh-cn.lmo` | Compiled English and Chinese catalogs |

### Translations

The source strings in the JS are Russian, so the Russian UI works without a catalog. `po/ru` is nearly empty and exists for tooling only. The English and Simplified Chinese catalogs cover every interface string.

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

The reply is `{ ok, data }` when the core printed JSON, otherwise `{ ok, code, output }`.

| ubus | Core command |
|------|--------------|
| `status`, `health`, `history`, `export_history`, `delay_history`, `metrics` | `rpc Status`, `Health`, `History`, `ExportHistory`, `DelayHistory`, `Metrics` |
| `validate`, `check_nft`, `check_fakeip`, `global_check` | `rpc Validate`, `CheckNft`, `CheckFakeip`, `GlobalCheck` |
| `switch_proxy` (`section`, `outbound`) | `rpc SwitchProxy <section> <outbound>` |
| `decode_uri` (`uri`) | `rpc DecodeURI <uri>` |
| `apply`, `reload` | `hybrid-failover apply`, `hybrid-failover reload` |
| `list_update`, `subscription_refresh`, `duplicate_section` | `rpc ListUpdate`, `SubscriptionRefresh`, `DuplicateSection` |
| `list_clients` | `rpc ListClients` |
| `backup_uci`, `backup_download`, `restore_uci` (`path`) | `rpc BackupUCI`, `BackupDownload`, `RestoreUCI` |
| `pending_capture`, `pending_validate`, `pending_apply`, `pending_rollback` | `rpc CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` |
| `update_check`, `update_status`, `update_apply` | `rpc UpdateCheck`, `UpdateStatus`, `UpdateApply` |
| `dhcp_leases` | No core call, see below |

The native engine has no Clash-style HTTP API. Status, channels and latency history come only through these RPCs. Latency history lives in `/var/run/hybrid-failover/delay-history.json`, the switch log in `/var/log/hybrid-failover/history.jsonl`.

`dhcp_leases` reads the dnsmasq lease files itself (paths from `leasefile` in UCI `dhcp`, otherwise `/tmp/dhcp.leases`) and falls back to `ubus call dhcp ipv4leases` if they are empty. Calling `luci-rpc` from inside rpcd deadlocked, which is why it is not used.

The Telegram tab bypasses this object. It calls `/usr/bin/hybrid-failover-bot -mode set-pending|validate-config|apply-config|rollback-config` through `fs.exec`, reads `/etc/hybrid-failover-bot.json` through `fs.read` and restarts the bot through `service restart`. The main ACL allows all of that.

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

The package postinst sets `luci.main.rpctimeout` to 60 seconds when it is unset or between 10 and 59 (Live probe and global-check can take longer than the default 20 seconds), restarts rpcd and uhttpd and clears the LuCI cache.

---

## Checking on the router

```sh
ubus call hybrid-failover status '{}'
hybrid-failover rpc Status
```

`scripts/luci-ubus-smoke.sh` walks through the main ubus methods and checks that each one answers.
