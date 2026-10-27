[Русский](../CLI.md)

# `hybrid-failover` commands

The core binary is `/usr/sbin/hybrid-failover`. LuCI and the bot do the same things by calling `hybrid-failover rpc <Method>`.

## Service

| Command | What it does |
|---------|--------------|
| `start` | Points dnsmasq at `127.0.0.42`, installs the nft rules, starts the engine, installs the list and subscription cron jobs |
| `stop` | Stops the engine, removes nft and cron, restores dnsmasq from backup |
| `restart` | `stop`, then `start` |
| `reload` | Re-reads UCI and updates the engine |
| `monitor` | The procd process: engine, failover controller and watchdog. Not run by hand |

Normally the service is managed with `/etc/init.d/hybrid-failover start|stop|restart|enable`.

## Configuration

| Command | What it does |
|---------|--------------|
| `migrate [--dry-run]` | Brings UCI up to the current schema, see [OVERVIEW.md](OVERVIEW.md#schema-migration). Prints `migration: no changes` when there is nothing to do |
| `validate [--dry-run] [--uci PATH]` | Checks URIs, `urltest_check_interval` / `urltest_idle_timeout` pairs, `list_route` bindings and builds the engine plan |
| `apply [--dry-run] [--uci PATH]` | Applies the engine plan and nft, reloads on changes |
| `pending capture` | Appends `uci changes hybrid-failover` to the snapshot in `/etc/hybrid-failover/pending` |
| `pending validate` | Checks the snapshot |
| `pending apply` | Commits staged changes (or replays the snapshot) and applies them |
| `pending rollback` | Deletes the snapshot |

`--uci PATH` checks or applies another file instead of `/etc/config/hybrid-failover`. How pending works is described in [OVERVIEW.md](OVERVIEW.md#pending-config-luci-and-bot).

## Lists and subscriptions

| Command | What it does |
|---------|--------------|
| `list-update` | Downloads community lists into `/etc/hybrid-failover/rulesets/` and updates the engine if they changed |
| `subscription-refresh [--dry-run] [--merge=false]` | Fetches links from `settings.subscription_urls` and writes them to `main_section` |

`subscription-refresh --dry-run` only downloads and prints the link count. By default manually added links are kept; `--merge=false` replaces the whole list. A `--section` flag exists, but when `settings.main_section` is set (the template sets it), `main_section` wins.

## Status and checks

| Command | What it does |
|---------|--------------|
| `status` | JSON: engine, nft, fakeip, active outbound, controller |
| `health` | Same plus a live channel probe |
| `global-check` | Same as `status` |
| `check-nft` (or `check`) | The `inet hybrid_failover` table is in place |
| `check-fakeip` | DNS query to `127.0.0.42`, expects a `198.18.x.x` address. No `dig` needed |
| `check-proxy [url]` | Plain HTTP GET from the router, `https://www.gstatic.com/generate_204` by default. Checks internet access, not a channel |

The `clash_ok` field in the report is kept for compatibility and mirrors `engine_running`.

## Updates

| Command | What it does |
|---------|--------------|
| `update check` | Latest GitHub release and whether an update is available |
| `update apply [--tag vX.Y.Z] [--force] [--foreground]` | Installs a release. Runs in the background by default, `--foreground` runs in the current console, `--force` allows the same or an older version |
| `update status` | Progress and log of the last update |

Details in [INSTALL.md](INSTALL.md#releases-and-updates).

## RPC

`hybrid-failover rpc <Method> [json]` prints JSON. A method can be written as `Status` or `status`, compound names as `ListRoutes` or `list_routes`.

| Method | Purpose |
|--------|---------|
| `Status`, `Health`, `GlobalCheck`, `CheckNft`, `CheckFakeip` | Status and checks |
| `Validate`, `Apply` | Check and apply UCI |
| `CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` | Pending config |
| `History`, `ExportHistory`, `DelayHistory`, `Metrics` | Switch log, delay history, channel counters |
| `SwitchProxy` | Manual switch of a section selector |
| `DecodeURI` | Show how core parsed a link |
| `ListClients` | Effective client rules |
| `ListRoutes` | Section channels and list bindings |
| `ListUpdate`, `SubscriptionRefresh`, `SubscriptionStatus` | Lists and subscriptions |
| `DuplicateSection` | Copy a routing section |
| `BackupUCI`, `BackupDownload`, `RestoreUCI` | UCI backup |
| `UpdateCheck`, `UpdateApply`, `UpdateStatus` | Updates |

Which of them LuCI calls is listed in [luci/README.en.md](../../luci/README.en.md).

## Logs and files

```sh
logread -e hybrid-failover                     # core system log
cat /var/log/hybrid-failover/history.jsonl     # switch log
cat /tmp/hybrid-failover/update/update.log     # last update log
```
