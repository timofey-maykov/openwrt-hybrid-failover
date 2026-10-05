[Русский](../UCI.md)

# UCI options (Hybrid Failover)

The configuration lives in **`/etc/config/hybrid-failover`**, and the UCI package is called **`hybrid-failover`**. The template with default values is [openwrt/etc/config/hybrid-failover](../../openwrt/etc/config/hybrid-failover).

Routing is described by `config section '<name>'` sections. The name is free-form and the engine builds its outbound tags from it. For a section called `glob` these are `glob-out` (the section selector), `glob-urltest-out` (the urltest group), `glob-awg-out` (the primary VPN interface when failover is on) and `glob-1-out`, `glob-2-out` and so on for the individual URIs in the list.

Only the native engine exists. The mode with an external sing-box was removed, and `hybrid-failover migrate` switches old installs to native. Options that only sing-box used to read are still in the template and in LuCI, but they have no effect. They are marked below.

The general description is in [OVERVIEW.md](OVERVIEW.md). Initial setup is done with `hybrid-failover migrate`.

---

## `config settings 'settings'`

The "Default" column shows what the code uses when the option is absent. If the template sets a different value, the description says so.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `engine_mode` | `native` | `native` | `apply` rejects any other value and asks you to run `migrate`. Migrate replaces `singbox` with `native` |
| `config_schema_version` | int | `0` | UCI schema version. The current `migrate` raises it to `5`. The template ships `4`, and the first `migrate` after install brings it to `5` |
| `bootstrap_dns_server` | IP | `77.88.8.8` | Upstream for the DNS server on `127.0.0.42`. Every A query that does not get a FakeIP goes here over plain UDP port 53. The same server is used when community lists are downloaded directly |
| `dns_rewrite_ttl` | int, seconds | `60` | TTL in FakeIP answers. Answers from the upstream always carry TTL 60 |
| `dns_type` | `doh` / `dot` / `udp` | `doh` | Goes into the engine plan, but the native engine DNS server does not use it at the moment |
| `dns_server` | string | `1.1.1.1` | Same as `dns_type`. Not used by the native engine at the moment |
| `disable_quic` | `0` / `1` | `0` | `1` rejects UDP/443 to FakeIP addresses with an nft rule and drops UDP/443 inside tproxy. Clients fall back to TCP |
| `disable_lan_ipv6` | `0` / `1` | `1` | Turns off RA and DHCPv6 on LAN and sets `filter_aaaa` in dnsmasq. Tproxy is IPv4 only, and without this Meta and Telegram can bypass the proxy over IPv6. `0` restores the saved LAN settings |
| `dont_touch_dhcp` | `0` / `1` | `0` | `1` leaves dnsmasq alone and does not point DNS at `127.0.0.42`. Without that redirect FakeIP does not work for LAN clients, so dnsmasq has to be configured by hand |
| `source_network_interfaces` | string | `br-lan` | Ingress interfaces for tproxy and DNS interception, separated by spaces. For a `br-*` bridge its ports are added too. The option is not in the template or in LuCI |
| `main_section` | string | `glob` | The main section. `status` and `health` show it, `subscription-refresh` writes into it, and lists are downloaded through it when `download_lists_via_proxy_section` is empty |
| `update_interval` | `1h` / `3h` / `12h` / `1d` / `3d` | `1d` | Cron period for `hybrid-failover list-update`. Other values are an error. The cron job is installed only if at least one section has lists |
| `download_lists_via_proxy` | `0` / `1` | `0` | `1` downloads community lists through a section. The engine starts an HTTP proxy on `127.0.0.1:1610` for this. The template sets `1` |
| `download_lists_via_proxy_section` | string | empty | Section used for list downloads. Empty means `main_section` |
| `webhook_url` | URL | empty | HTTP webhook for failover events. The old name `failover_webhook_url` is read when `webhook_url` is empty |
| `failover_probe_interval` | duration | `30s` | Interval of the background failover controller (not the urltest interval). Values below `15s` are replaced with `30s`, values above `5m` are capped at `5m` |
| `history_max_lines` | int | `500` | How many lines to keep in `/var/log/hybrid-failover/history.jsonl`. Allowed range is 50 to 10000, anything below 50 means 500 |
| `delay_history_points` | int | `50` | Delay points per channel in `/var/run/hybrid-failover/delay-history.json`, from 10 to 200 |
| `output_network_interface` | string | empty | Read into the engine plan, but the native engine does not apply it |
| `list subscription_urls` | list | - | Subscription URLs. `hybrid-failover subscription-refresh` or LuCI write the links into `main_section` |
| `subscription_update_interval` | `off` / `1h` / `3h` / `6h` / `12h` / `1d` | `off` | Cron period for `hybrid-failover subscription-refresh`. The job exists only when `subscription_urls` is set. A refresh replaces the links the previous refresh wrote (remembered in `/etc/hybrid-failover/subscription-links.json`) and keeps the links added by hand. A subscription with no supported links changes nothing |
| `list include_source_ips` | list | - | Legacy, see `client_rule` |
| `list exclude_source_ips` | list | - | Legacy, see `client_rule` |
| `list routing_excluded_ips` | list | - | Legacy, see `client_rule` with `mode global_exclude` |

### Options with no effect in the native engine

These options are in the template and in LuCI, but the native engine does not use them. Apart from `enabled`, only the legacy sing-box code and the Clash API address lookup read them. The native engine has no Clash API HTTP server, and nothing listens on `:9090`.

| Option | Template | What it used to do |
|--------|----------|--------------------|
| `enabled` | `1` | A flag in LuCI. The code does not read it, the service is enabled with `/etc/init.d/hybrid-failover enable` |
| `cache_path` | `/etc/sing-box/cache.db` | sing-box cache file. `migrate` still sets it when the option is missing |
| `clash_api_listen` | `127.0.0.1:9090` | sing-box Clash API address |
| `service_listen_address` | empty | Fallback Clash API address when `clash_api_listen` is empty |
| `enable_yacd` | `0` | Yacd web UI in sing-box |
| `enable_yacd_wan_access` | `0` | Clash API on `0.0.0.0:9090` |
| `yacd_secret_key` | empty | Bearer secret for the Clash API |

---

## `config client_rule '<name>'`

Rules for individual LAN clients. A rule sets the client IP and how its traffic is handled. In LuCI this is the **Hybrid Failover → Clients** tab, with a step-by-step guide in [LUCI.md](LUCI.md).

| Option | Type | Description |
|--------|------|-------------|
| `ip` | string | Client IP or CIDR, for example `192.168.11.236`. A rule without `ip` is skipped |
| `mode` | string | Mode, see the table below |
| `section` | string | Routing section. Only needed for `full_route` |

### `mode` values

| Value | Aliases | What happens |
|-------|---------|--------------|
| `include` | `in` | All client traffic is marked and goes into tproxy. Engine rules decide where it goes next |
| `exclude` | `out` | Client DNS is not intercepted and traffic does not enter tproxy by subnet. FakeIP addresses of community domains still go through the proxy if the client asks the router for DNS. Do not put Xbox or PS consoles here, use `subnet_bypass_ips` for them |
| `full_route` | `full`, `fully_routed` | Client traffic is marked as with `include`, and the engine sends everything from this IP to `section`, ahead of the list rules |
| `global_exclude` | `routing_excluded` | Only worked in sing-box, as a destination exclusion from routing. The native engine does not handle this mode |

After changing rules LuCI reloads on its own. From the console run `hybrid-failover reload`.

Check the result with `hybrid-failover rpc ListClients` or the **Refresh** button on the **Clients** tab (the Effective rules table).

### Legacy lists

As long as the config has no `client_rule` section at all, core reads the old lists.

| UCI | Mode |
|-----|------|
| `settings.include_source_ips` | `include` |
| `settings.exclude_source_ips` | `exclude` |
| `settings.routing_excluded_ips` | `global_exclude` |
| `section.<name>.fully_routed_ips` | `full_route` for that section |

`hybrid-failover migrate` (schema v2) moves them into `client_rule`. Once there is at least one `client_rule` section, the old lists are no longer read. Add new rules through `client_rule` or the **Clients** tab.

---

## `config section '<name>'`

### Common

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `connection_type` | `vpn` / `proxy` / `block` | - | Section type |
| `enabled` | `0` / `1` | `1` | `0` removes the section from the engine plan. nft rules from its `udp_routed_ips`, `subnet_bypass_ips` and subnets stay in place |

### `connection_type 'vpn'`

| Option | Type | Description |
|--------|------|-------------|
| `interface` | string | VPN interface, for example `awg0`. Required |
| `failover_vpn_enabled` | `0` / `1` | Enables backup proxies for the VPN |
| `list failover_proxy_links` | list | Backup URIs. Formats are described in [OVERVIEW.md](OVERVIEW.md), in the section on supported links |
| `failover_policy` | string | `outage-only` (default), `prefer-primary` or `fastest`. `latency` and `urltest` are read as `fastest` |
| `failover_fail_threshold` | int | Consecutive failed VPN checks before switching to backups. Default 2 |
| `failover_recover_threshold` | int | Consecutive successful checks before returning to the VPN. Default 1 for `prefer-primary` and 2 for the others |

If `failover_vpn_enabled=0` or the backup list is empty, the section gets a single outbound bound to `interface`.

The order of URIs in `failover_proxy_links` is the order of backups in urltest, after the primary VPN.

**`failover_policy` values**

| Value | Behaviour |
|-------|-----------|
| `outage-only` | Traffic goes through `interface` while checks pass. After a run of failures the controller switches the selector to urltest over the backups. Returns to the VPN after 2 successful checks |
| `prefer-primary` | Like `outage-only`, but returns after 1 successful check |
| `fastest` | VPN and backups share one urltest, and the engine picks the fastest channel. The failover controller does not interfere with switching |

The controller runs in the background of core, checks the channels and switches the selector through the engine control channel. It does not need a Clash API.

### `connection_type 'proxy'`

| Option | Type | Description |
|--------|------|-------------|
| `proxy_config_type` | `url` / `urltest` / `outbound` | Configuration type, `url` by default |
| `proxy_string` | string | A single link, with `proxy_config_type=url` |
| `list urltest_proxy_links` | list | List of URIs, with `proxy_config_type=urltest`. Several `awg2://` links with the same `public_key` and different IPs make one interface with endpoint rotation. Amnezia 3.1 needs kmod and tools 3.1 or newer, otherwise `RandomTrailers` breaks the handshake. Do not mix a dead AWG 2.0 peer and a live 3.1 peer in one list |
| `outbound_json` | string | Outbound JSON, with `proxy_config_type=outbound`. The native engine accepts only `"type": "socks"` (fields `server`, `server_port`) and `"type": "direct"` (field `bind_interface`) |

### `connection_type 'block'`

A section without an outbound. The engine rejects traffic that matches its lists.

### Domain and subnet lists

If a section has at least one list, it is list-based. The engine sends only matching traffic into it and everything else goes direct. A `vpn` or `proxy` section without lists takes all traffic that entered tproxy and did not match other rules.

| Option | Type | Description |
|--------|------|-------------|
| `list community_lists` | list | Ready-made lists from [itdoginfo/allow-domains](https://github.com/itdoginfo/allow-domains), for example `russia_inside`, `youtube`, `discord`, `telegram`, `meta`. The domains get FakeIP. For `twitter`, `meta`, `discord`, `roblox`, `telegram`, `cloudflare`, `hetzner`, `ovh`, `digitalocean` and `cloudfront` IPv4 subnets are downloaded as well. Files are stored in `/etc/hybrid-failover/rulesets/` |
| `user_domain_list_type` | `disabled` / `dynamic` / `text` | Source of custom domains |
| `list user_domains` | list | Domains with `dynamic` |
| `user_domains_text` | string | Domains with `text`, separated by newline, space or comma |
| `list local_domain_lists` | list | Paths to domain list files on the router |
| `user_subnet_list_type` | `disabled` / `dynamic` / `text` | Source of custom subnets |
| `list user_subnets` | list | Subnets with `dynamic` |
| `user_subnets_text` | string | Subnets with `text`, same separators |
| `list local_subnet_lists` | list | Subnet files (`.lst` or JSON ruleset) |
| `list remote_domain_lists` | list | Domain list URLs. The native engine neither downloads nor applies them |
| `list remote_subnet_lists` | list | Subnet list URLs in `.lst` format |
| `list subnet_bypass_ips` | list | Client IPs, usually consoles. FakeIP traffic and community domains go through the proxy, while subnets from `hf_proxy_subnets` and Teredo stay on WAN. Pin these IPs with a static DHCP lease |
| `list udp_routed_ips` | list | Client IPs whose UDP, except DNS, goes into tproxy and into this section. TCP goes direct. Works for `vpn` and `proxy` sections |

Custom subnets (`user_subnets`, `user_subnets_text`, `local_subnet_lists`, `remote_subnet_lists`) end up in the nft set `hf_proxy_subnets`, and traffic to them enters tproxy. For `user_subnets` and `user_subnets_text` the native engine builds a route rule into the section. There is no rule for `local_subnet_lists` and `remote_subnet_lists` yet, so the engine sends that traffic direct. Subnets from `community_lists` and from `user_list` are routed into the section as expected.

| Option | Type | Description |
|--------|------|-------------|
| `list channel_names` | list | Your own channel names as `id=Name`. The channel ids are shown on the "Сервисы и каналы" (Services and channels) tab, by `/routes` in the bot and by `hybrid-failover rpc ListRoutes` |

### Section channels

Every link in `urltest_proxy_links` (or in `failover_proxy_links` of a VPN with backups) becomes a channel. A VPN section has the interface itself as its first channel, with the id `vpn`. The id of any other channel is the first 8 characters of a sha256 over the link's server and credentials: the peer public key for `awg2://`, the scheme, user, host and port for the rest. Obfuscation parameters and the order of the links do not change the id, so bindings survive editing and reordering links. Two `awg2://` links to the same peer with different addresses make one channel.

## `config user_list '<name>'`

Your own named list of domains and subnets. It works like a separate service and can be bound to its own channel. A section with at least one enabled `user_list` routes by lists.

| Option | Type | Description |
|--------|------|-------------|
| `section` | string | Routing section (`vpn` or `proxy`) the list belongs to |
| `title` | string | Name shown in the interface and the bot |
| `enabled` | bool | Defaults to `1` |
| `domains_text` | string | Domains separated by newlines, spaces, commas or semicolons. Subdomains are included |
| `list domains` | list | The same domains as a list |
| `subnets_text` | string | IPv4 subnets or addresses, a single address counts as `/32` |
| `list subnets` | list | The same as a list |

The subnets go into `hf_proxy_subnets`, the domains get FakeIP. Bindings refer to the list as `user:<name>`.

## `config list_route '<name>'`

Binds one or more lists of a section to a channel. A list with no binding goes through the section pool, which is the urltest choice, as before. Binding rules are checked before the section rule, so a binding wins when domains overlap. When two bindings name the same list, the first one in the file applies.

| Option | Type | Description |
|--------|------|-------------|
| `section` | string | Routing section |
| `list lists` | list | List keys: a `community_lists` name (`youtube`), `user` for the section's own domains and subnets, `local` for `local_domain_lists`, `user:<name>` for a `user_list` |
| `channel` | string | Channel id, `auto` (the pool, same as no binding), `balance` (spread over all channels), `direct` (no tunnel) or `block` |
| `on_down` | `pool` / `direct` / `block` | Where the lists go while the bound channel fails its urltest. Defaults to `pool` |
| `enabled` | bool | Defaults to `1` |

A channel counts as down when its last urltest probe failed. Before the first probe it counts as alive. When a connection through the channel cannot be opened, the engine tries the next `on_down` target right away instead of waiting for the probe.

With `balance` the engine spreads connections over the live channels of the section, sticky per site: every connection to one site (the last two labels of the name, or the IP) uses one channel while that channel lives. Sites that check the session IP keep working. When the bound channel is removed from the section, its lists go back to the pool, and `validate` and the services tab warn about it.

Example:

```
config user_list 'ul_work'
	option section 'main'
	option title 'Work'
	option domains_text 'jira.example.com gitlab.example.com'
	option subnets_text '10.20.0.0/16'

config list_route 'lr_main_youtube'
	option section 'main'
	list lists 'youtube'
	option channel 'a1b2c3d4'
	option on_down 'pool'

config list_route 'lr_main_user_ul_work'
	option section 'main'
	list lists 'user:ul_work'
	option channel 'direct'
```

### urltest parameters

Used for a VPN with backups and for a proxy with `proxy_config_type=urltest`.

| Option | Default | Description |
|--------|---------|-------------|
| `urltest_check_interval` | `3m` | Check interval. Give it with a unit (`30s`, `3m`). The engine cannot parse a bare number and falls back to `30s`. Must not be greater than `urltest_idle_timeout`. The template sets `30s` |
| `urltest_tolerance` | `50` | Delay tolerance in ms. The node changes only if the new one is faster by at least this much |
| `urltest_testing_url` | `https://www.gstatic.com/generate_204` | URL used for checks |
| `urltest_idle_timeout` | empty | Only checked as a pair with `urltest_check_interval` during `validate` and `apply`. The engine itself does not use the value |
| `urltest_interrupt_exist_connections` | `0` | Goes into the plan, but the native engine does not drop existing connections when the node changes |

### Section options with no effect in the native engine

| Option | What it used to do |
|--------|--------------------|
| `enable_udp_over_tcp` | UDP over TCP for SS and SOCKS in sing-box. The native engine reads the option and ignores it |
| `domain_resolver_enabled` | A separate DNS for the outbound server name in sing-box |
| `domain_resolver_dns_type` | Type of that DNS (`doh`, `dot`, `udp`) |
| `domain_resolver_dns_server` | Address of that DNS |
| `list fully_routed_ips` | Replaced by `client_rule` with `mode full_route`, read only while there is no `client_rule` |

---

## Bot service UCI (`/etc/config/hybrid-failover-bot`)

Section `config bot 'main'`. The init script `/etc/init.d/hybrid-failover-bot` reads these options.

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `0` | `1` starts the bot |
| `binary` | `/usr/bin/hybrid-failover-bot` | Path to the binary |
| `config_path` | `/etc/hybrid-failover-bot.json` | Path to the bot JSON config |
| `log_path` | `/var/log/hybrid-failover-bot.log` | Log file, passed to the bot through `HF_BOT_LOG_PATH` |

---

## CLI and validation

```sh
hybrid-failover migrate [--dry-run]    # import old UCI and migrate the schema to v5
hybrid-failover validate [--dry-run]   # check UCI and build the engine plan
hybrid-failover apply [--dry-run]      # apply the engine plan and nft, reload on changes
hybrid-failover reload                 # re-read UCI
hybrid-failover check-fakeip           # DNS through 127.0.0.42 without bind-dig
hybrid-failover list-update            # update community lists
hybrid-failover subscription-refresh   # write subscription links into main_section
```

`validate` and `apply` check the URIs in `failover_proxy_links` and `urltest_proxy_links` and the `urltest_check_interval` / `urltest_idle_timeout` pair.

---

## Example

See [examples/glob-uci-commands.txt](../../examples/glob-uci-commands.txt) and the template [openwrt/etc/config/hybrid-failover](../../openwrt/etc/config/hybrid-failover).
