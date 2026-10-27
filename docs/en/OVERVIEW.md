[Русский](../OVERVIEW.md)

# Hybrid Failover overview

A self-contained routing stack for OpenWrt. The `hybrid-failover` binary ships its own **proxy engine** (tproxy, DNS with fakeip, outbounds, urltest). It also supports Amnezia `vpn://` links, an extended URLTest, a Telegram bot and a LuCI app.

**Core:** `/usr/sbin/hybrid-failover` · **UCI:** `/etc/config/hybrid-failover` · **nft:** `inet hybrid_failover`

---

## Contents

1. [Architecture](#architecture)
2. [Routing modes](#routing-modes)
3. [Supported link formats (URI)](#supported-link-formats-uri)
4. [UCI options](#uci-options)
5. [Per-device rules](#per-device-rules)
6. [Switch notifications](#switch-notifications)
7. [Diagnostics](#diagnostics)
8. [Troubleshooting](#troubleshooting)

Installation is covered in [INSTALL.md](INSTALL.md), the web interface in [LUCI.md](LUCI.md), every option in [UCI.md](UCI.md), the commands in [CLI.md](CLI.md), the bot in [bot/README.en.md](../../bot/README.en.md).

---

## Architecture

![Hybrid Failover diagram](../img/hybrid-failover-schema.en.png)

The Go core `hybrid-failover` reads UCI and compiles it into a plan for the built-in engine. It then sets up nft tproxy (table `inet hybrid_failover`, mark `0x105`, port 1602) and runs the engine inside its own process. procd supervises `hybrid-failover monitor`. That process hosts the engine, the failover policy controller and the watchdog.

There is no external sing-box anymore. The only mode is `settings.engine_mode=native`. The old `engine_mode=singbox` has been removed, and `hybrid-failover migrate` switches such configs to native. The `internal/singbox` package is still in the tree as a set of shared constants, tag naming and list parsing, and for migration. No traffic goes through it.

LuCI and the bot talk to the core through `hybrid-failover rpc <Method>`. LuCI reaches it via rpcd (the ucode script `/usr/share/rpcd/ucode/hybrid-failover`). A manual channel switch is handed to the running engine through IPC files in `/var/run/hybrid-failover`. The native engine has no Clash API HTTP server, so nothing listens on port 9090.

```mermaid
flowchart TB
  subgraph vpn_failover [VPN + failover]
    AWG["{section}-awg-out<br/>direct + bind_interface"]
    P1["{section}-1-out"]
    P2["{section}-2-out"]
    UT["{section}-urltest-out"]
    SEL["{section}-out<br/>selector, defaults to urltest"]
    AWG --> UT
    P1 --> UT
    P2 --> UT
    UT --> SEL
  end
  Client[LAN traffic] --> SEL
```

A vpn section such as `glob` gets the following outbounds. The name is arbitrary and becomes the tag prefix. A proxy section with `urltest_proxy_links` gets the same set without `{section}-awg-out`: the `{section}-1-out`, `{section}-2-out` and further channels come from `urltest_proxy_links`.

| Outbound | Purpose |
|----------|---------|
| `{section}-awg-out` | The section's VPN interface, **direct** through `option interface` (e.g. `awg0`). With `outage-only` and `prefer-primary` it is the primary channel |
| `{section}-1-out`, `{section}-2-out`, ... | Proxies from `failover_proxy_links`, one per URI |
| `{section}-urltest-out` | **urltest** over all channels of the section |
| `{section}-out` | **selector**, points at urltest by default |

The order of URIs in the list sets candidate priority after the primary VPN. Within urltest the live and fastest one wins.

Every core command is listed in [CLI.md](CLI.md).

### DNS via 127.0.0.42

On `start` the core points dnsmasq at the engine DNS unless `settings.dont_touch_dhcp=1` is set.

1. **dnsmasq.** The current settings are saved to `/etc/hybrid-failover/dnsmasq-dhcp.bak`, then `noresolv=1` and `server=127.0.0.42` are applied.
2. **Engine DNS.** Listens on `127.0.0.42:53`, hands out fakeip addresses from `198.18.0.0/15` and honours the community lists. Other names are resolved with a plain UDP query to `settings.bootstrap_dns_server` (77.88.8.8 by default). The native engine does not use `dns_type` and `dns_server` at the moment.
3. **Check.** `hybrid-failover check-fakeip` asks `127.0.0.42` for `fakeip.hybrid-failover` and expects a `198.18.x.x` address.

On `stop` dnsmasq is restored from the backup. At boot the init script first puts dnsmasq back on its normal upstreams if `127.0.0.42` was left there, so the LAN keeps DNS while the engine is still starting.

```mermaid
flowchart LR
  LAN[LAN clients] --> dnsmasq
  dnsmasq -->|"127.0.0.42:53"| engineDNS[engine DNS]
  engineDNS --> fakeip[fakeip 198.18.0.0/15]
  engineDNS --> upstream["bootstrap DNS, UDP 53"]
  router[Router itself] --> dnsmasq
```

### Pending config (LuCI and bot)

LuCI and the bot do not commit UCI right away. They stage changes (`uci set` without `commit`), and the core keeps a snapshot of them in `/etc/hybrid-failover/pending`.

| Step | CLI | What it does |
|------|-----|--------------|
| Capture | `pending capture` | Reads `uci changes hybrid-failover` and merges them into the snapshot |
| Validate | `pending validate` | Checks keys and values in the snapshot |
| Apply | `pending apply` | Commits the staged changes, recompiles the plan and reloads the engine |
| Rollback | `pending rollback` | Deletes the snapshot |

The snapshot covers the case where staging is lost, for example when a reboot wipes `/tmp/.uci`. Then `pending apply` replays the changes from the snapshot. If the staged changes are still there, those are committed and the snapshot is not applied on top.

The same steps are exposed as the RPC methods `CapturePending`, `PendingValidate`, `PendingApply` and `PendingRollback`. On the LuCI routing page "Save" runs capture and "Apply" calls `pending_apply`.

### Lists and cron

- `list-update` downloads community lists into `/etc/hybrid-failover/rulesets/`. When they change, the core recompiles the plan and refreshes the engine without a full restart.
- `start` also runs a list update.
- `update_interval` in the `settings` section (default `1d`) controls a cron job running `hybrid-failover list-update`. The job is installed on `start` when the config has lists and removed on `stop`.

---

## Routing modes

### 1. VPN + failover

| UCI | Value |
|-----|-------|
| `connection_type` | `vpn` |
| `failover_vpn_enabled` | `1` |
| `failover_proxy_links` | list of URIs (see the table below) |
| `interface` | VPN interface, e.g. `awg0` |

With `failover_policy` `outage-only` and `prefer-primary` traffic goes through the VPN interface, the proxies from the list are used only while it is down, and traffic returns to the VPN once it recovers. With `fastest` there is no primary: urltest keeps the fastest live channel, and the VPN interface competes with the proxies on equal terms.

### 2. Proxy with URLTest

| UCI | Value |
|-----|-------|
| `connection_type` | `proxy` |
| `proxy_config_type` | `urltest` |
| `urltest_proxy_links` | list of URIs |

Proxies only, no `bind_interface`. URI formats and urltest options are the same.

### 3. Own channels for lists

When a section has several channels, service lists can be spread over different tunnels so YouTube does not share a channel with everything else. Bindings are set on the "Сервисы и каналы" (Services and channels) tab or with `/route` in the bot and are stored in `config list_route`. A list with no binding goes through the section pool, as in the modes above. For each binding you choose what happens when its channel is down: fall back to the pool, go direct, or stay blocked. `balance` spreads the list's connections over all live channels while keeping each site on one channel. Your own named lists of domains and subnets (`config user_list`) are bound the same way as the ready-made ones. Details are in [UCI.md](UCI.md#config-list_route-name).

### URLTest parameters

The check interval, latency tolerance and probe URL are set with the section's `urltest_*` options, see [UCI.md](UCI.md#urltest-parameters). It also marks which of them the native engine does not use yet.

---

## Supported link formats (URI)

Links are parsed in Go (`internal/uri`). Amnezia `vpn://` is decoded by `internal/amnezia`, no Python needed.

### In `failover_proxy_links` and `urltest_proxy_links`

| Scheme | Supported | Notes |
|--------|-----------|-------|
| `vless://` | yes | Reality and TLS, XTLS Vision, TCP and UDP (XUDP). Transports tcp, ws and grpc, the others (httpupgrade, xhttp, h2) are accepted but the channel shows a "transport is not supported" error. These link parameters are used: `sni`, `alpn`, `fp`, `allowInsecure`, `path`, `host`, `serviceName`, `authority`. Vision works on plain TCP only, not over ws or grpc |
| `ss://` | yes | Shadowsocks |
| `trojan://` | yes | |
| `socks4://`, `socks4a://`, `socks5://` | yes | |
| `hysteria2://`, `hy2://` | yes | TLS (`sni`, `insecure`), obfs, `up`/`down` in mbps |
| `vpn://` | yes | **Amnezia** export, converted to `vless://` (xray) or `awg2://` (awg, awg2) |
| `awg2://` | yes (internal URI) | Not a protocol, see [below](#amnezia-awg2-awg2) |
| `amneziawg://` | subscriptions only | A whole AmneziaWG client config in base64, as subscription panels hand it out. Converted to `awg2://` when the subscription is loaded, the first peer is used |
| `http://`, `https://` | no | Fails with `unsupported uri scheme` |

### Amnezia `vpn://`

The decoder is built into the core (`internal/amnezia`). It handles the usual **amnezia-xray** export with VLESS in `last_config`. In LuCI you can paste a `vpn://...` string as is.

### Amnezia AWG2 (`awg2://`)

`awg2://` is not a network protocol of its own. It is an internal core format for setting up **AmneziaWG**, including 3.1. The core does three things with it.

1. Creates an interface of type `amneziawg`.
2. Applies the config to it with `awg setconf`.
3. Adds a direct outbound bound to that interface to the engine plan.

An `awg2://...` string appears when a `vpn://` link is converted and the Amnezia container is `amnezia-awg2` or `amnezia-awg`. In LuCI you can paste the `vpn://` link directly.

The decoder carries the 3.1 fields into the `awg2://` query. These are `header_protection_key`, `content_padding_addition`, `random_trailers`, `disable_cookies`, the timers (`rekey_after_time`, `rekey_timeout`, `reject_after_time`, `keepalive_timeout`, `max_handshake_attempts`) and `persistent_keepalive` as a range (`25-35`).

#### Packages on the router

The core only writes the config. The tunnel itself is brought up by `kmod-amneziawg` and `amneziawg-tools`. Amnezia 3.1 exports (`protocol_version: 3.1`, `RandomTrailers`) need version 3.1 or later of both. They must be built for the same OpenWrt release and kernel vermagic as the router (`opkg info kernel`). Prebuilt `.ipk` files are often published in the [2Grey/awg-openwrt](https://github.com/2Grey/awg-openwrt) releases, tagged by OpenWrt version (for example `v24.10.6`).

```sh
awg --version          # should be 3.1.x, not 1.0.20260618
awg set --help | grep random-trailers
```

The 3.0 tools reject the config with `Line unrecognized: RandomTrailers=on`, and the interface stays down.

#### Handshake and parameter mismatch

`S1-S4`, `H1-H4`, `HeaderProtectionKey` and `RandomTrailers` must match the server. `RandomTrailers=on` works in both directions. A 3.1 server pads handshake packets, and a 3.0 client does not recognise them and silently drops them. From the outside this looks odd. conntrack shows the UDP flow as `ASSURED` with replies coming in, yet `awg show` reports `0 B received` and no handshake.

`DisableCookies`, `Jc`, `Jmin`, `Jmax` and `ContentPaddingAddition` do not have to match.

An old AWG 2.0 profile (ranges in `H1-H4`, `I1`, no `HeaderProtectionKey`) will not get a handshake from a 3.1 server port. Don't keep such URIs in `urltest_proxy_links` next to 3.1 ones, or the watchdog will keep cycling endpoints of a dead peer.

One `vpn://` link cannot be used on the router and in the Amnezia app on a LAN device at the same time. An AWG key gives one session. The phone takes over the endpoint and the router is left without a handshake.

```sh
awg show               # latest handshake, transfer
```

In urltest mode the channel card in LuCI looks at the HTTP check (`urltest_testing_url`) first. For AWG the handshake is taken into account on top of that. A fresh handshake with a failing HTTP check shows as "handshake есть, HTTP urltest не проходит" (the UI text is Russian). With no handshake, or one older than three minutes, the channel is shown as DOWN with a reason.

Traffic goes through the selector. While the selector points at urltest, the live channel is whatever wins the HTTP check, often Hysteria. An AWG handshake alone does not switch urltest. A manual "Switch" pins the selector to a specific outbound. The `SwitchProxy` command reaches the monitor process over IPC and does not wait for the next controller poll.

### Links added through the Telegram bot

The bot validates links with the same function as the core (`internal/validation`). It accepts `vless`, `trojan`, `ss`, `vpn`, `socks4`, `socks4a`, `socks5`, `hysteria2`, `hy2` and `awg2`.

---

## UCI options

The full table is in [`UCI.md`](UCI.md), example commands in [`examples/glob-uci-commands.txt`](../../examples/glob-uci-commands.txt).

The config lives in `/etc/config/hybrid-failover`. The installer, the core package postinst and `hybrid-failover update apply` run `hybrid-failover migrate`, so there is no need to run it by hand.

### Schema migration

On its first run `migrate` imports the previous UCI config if the new one does not exist yet. It then brings `settings.config_schema_version` up to 5.

| Schema | Changes |
|--------|---------|
| v1 | `failover_vpn_enabled=0` if the option was missing. `failover_vpn_enabled=1` for a VPN section that already has `failover_proxy_links`. `urltest_interrupt_exist_connections=0` if unset |
| v2 | Legacy client lists are moved into `client_rule` sections |
| v3, v4 | `settings.engine_mode=native`, including replacing `singbox` |
| v5 | `settings.disable_lan_ipv6=1` if the option is unset |

Migration v1 also writes `settings.cache_path`. That option is a sing-box leftover and the native engine ignores it.

---

## Per-device rules

Global routing (sections, domain lists) decides which traffic goes to the engine. Client rules decide which LAN devices take part and how.

In UCI these are `config client_rule` sections, in LuCI the **Клиенты** (Clients) tab.

| `mode` | Behaviour |
|--------|-----------|
| `include` | The client's traffic goes into the engine, where the section rules sort it |
| `exclude` | The client's DNS is not intercepted and subnet traffic bypasses the engine. FakeIP domains still go through the proxy if the client asks the router for DNS |
| `full_route` | All of the client's traffic goes through the given section (`option section`) |
| `global_exclude` | Only worked with sing-box. The native engine does not handle this mode |

Synonyms and details are in [UCI.md](UCI.md#config-client_rule-name). `hybrid-failover rpc ListClients` shows the effective rules. An empty list with the core running means there are no rules. It is not an error.

Legacy lists (`settings.include_source_ips`, `exclude_source_ips`, `routing_excluded_ips`, per-section `fully_routed_ips`) are moved into `client_rule` by `migrate`. As long as at least one `client_rule` exists, the legacy lists are ignored.

```sh
# Example: route a console at 192.168.1.50 through HF
uci set hybrid-failover.console=client_rule
uci set hybrid-failover.console.ip='192.168.1.50'
uci set hybrid-failover.console.mode='include'
uci commit hybrid-failover
hybrid-failover reload
```

Adding a rule in the web interface is covered in [LUCI.md](LUCI.md#clients).

---

## Switch notifications

The core writes every switch to `/var/log/hybrid-failover/history.jsonl` (`settings.history_max_lines` sets its size).

**From the bot.** Set `notify_failover_enabled: true` in `/etc/hybrid-failover-bot.json` and the bot sends new events to its admins. Details are in [bot/README.en.md](../../bot/README.en.md#failover-notifications).

**From a core webhook.** Put an HTTP endpoint into `hybrid-failover.settings.webhook_url`. On every automatic switch the core sends a `POST` with the event as a JSON body.

```json
{"time":"2026-09-29T10:15:00Z","section":"glob","from":"glob-awg-out","to":"glob-1-out","reason":"primary outage","policy":"outage-only"}
```

Any relay of your own can receive it and, for example, forward the text to Telegram with `sendMessage`. The core only makes the `POST` and expects a 2xx reply.

---

## Diagnostics

The engine has no HTTP API of its own. State is available through the CLI, RPC and the system log.

```sh
hybrid-failover status          # JSON: engine, nft, fakeip, active outbound, controller
hybrid-failover health          # same plus live channel probes
hybrid-failover check-fakeip    # DNS query to 127.0.0.42, no dig needed
hybrid-failover check-nft       # inet hybrid_failover table is in place
logread -e hybrid-failover
```

The other commands are in [CLI.md](CLI.md).

In the bot, `/status`, `/health` and `/channels` show the same data.

---

## Troubleshooting

### Empty table of effective rules on the Clients tab

The core may be running fine (`engine_running: true` in the status). The message means there are no `client_rule` sections. Add a rule on the Clients tab or through UCI, see [LUCI.md](LUCI.md#clients).

### No leases, empty DHCP picker

The picker reads the dnsmasq lease file (`/tmp/dhcp.leases`).

```sh
cat /tmp/dhcp.leases
/etc/init.d/dnsmasq status
```

On OpenWrt `ubus call dhcp ipv4leases` only returns data when odhcpd is the main DHCPv4 server. dnsmasq leases never show up there.

### check-fakeip fails or the LAN has no DNS

Make sure the engine is running and dnsmasq points at it.

```sh
hybrid-failover status
uci -q get dhcp.@dnsmasq[0].server   # should be 127.0.0.42
hybrid-failover check-fakeip
logread -e hybrid-failover | tail -50
```

If a stale dnsmasq instance holds `127.0.0.42:53`, run `/etc/init.d/hybrid-failover restart`. The core checks for this case on start.

### Double failover

If you are moving to Hybrid Failover, remove third-party automatic failover scripts and disable conflicting routing init.d services. `hybrid-failover migrate` warns about the conflicts it finds.

---

The repository layout is described in [README.en.md](../../README.en.md#repository-layout).
