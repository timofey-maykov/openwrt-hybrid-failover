[Русский](README.md)

# Hybrid Failover Bot

A Telegram bot for managing Hybrid Failover and the OpenWrt router itself (state, network, Wi-Fi, services, port forwards, updates). It shows status and channels, edits the `hybrid-failover` UCI config through pending changes, applies them, sends notifications when a channel switches, and keeps an eye on the router: when something breaks it restarts the service and tells you if that did not help. Status, channel checks, history and apply all go through core RPC (`hybrid-failover rpc`), so the bot behaves the same on the local router and on a remote one over SSH.

Every installation gets its own bot, with a token from [@BotFather](https://t.me/BotFather). In LuCI the settings live under **Services → Hybrid Failover → Telegram**. Installation is covered in [docs/en/INSTALL.md](../docs/en/INSTALL.md), and the packages are in [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases).

The bot is a single binary, `/usr/bin/hybrid-failover-bot`, with no Python or Node. Besides the Go standard library it uses telegram-bot-api and `x/crypto/ssh`.

## Building

`./scripts/build-packages.sh` builds packages for all architectures (see [packages/README.en.md](../packages/README.en.md)). To build the binary by hand, for example for `mipsel_24kc`:

```sh
cd bot
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags="-s -w" -o hybrid-failover-bot ./cmd/hybrid-failover-bot
```

The bot module pulls in the core with `replace ../`, so build it from a full checkout of the repository.

## Config

The config file is `/etc/hybrid-failover-bot.json`. The package ships a single-router version of it, without a `routers` list.

`token` and `admin_ids` are required. The token can also come from the `HF_BOT_TOKEN` environment variable, which wins over the file.

| Key | Default | Meaning |
|-----|---------|---------|
| `router_name` | hostname | router name shown in `/panel`, `/status` and failover notifications |
| `viewer_ids` | `[]` | read-only users, see below |
| `allow_shell_ids` | `[]` | who may run `/sh`, empty means off, see "Router management" |
| `notify_failover_enabled` | `false` | send new failover events to admins |
| `notify_failover_interval_seconds` | `30` | how often to check the history. `/config_set` and LuCI refuse values under 10, in the JSON itself zero becomes 30 |
| `log_path` | `/var/log/hybrid-failover-bot.log` | bot log |
| `audit_path` | `/var/log/hybrid-failover-bot.audit.log` | command log with user ID and result |
| `routing_init_script` | `/etc/init.d/hybrid-failover` | what `/routing_restart` calls |
| `uci_package` | `hybrid-failover` | UCI package |
| `main_section` | `glob` | section used by `/set_*`, `/failover_*` and the key aliases |
| `probe_timeout_seconds` | `5` (15 in the shipped file) | timeout for HTTP probes through the Clash API |
| `clash_api` | `http://127.0.0.1:9090` | fallback for old installs with an external sing-box. The native engine has no Clash API, and the bot reads channels from core RPC |
| `policy` | `outage-only` | validated and shown by `/config_show`, has no effect on routing. The failover policy is set with `/set_policy` |
| `routers` | none | router list for the one-bot-many-routers setup |
| `watchdog_enabled` | `true` | router monitoring, see "Monitoring and self-repair" |
| `watchdog_auto_repair` | `true` | let the bot restart the service on a failure |
| `watchdog_interval_seconds` | `30` | how often to check the routers, under 10 becomes 30 |
| `update_check_enabled` | `true` | ask core about a new release once a day and offer it, see "Updates" |
| `update_check_interval_hours` | `24` | how often to check for releases, at least 1 |

If `main_section` does not exist in the local router's UCI, the bot falls back to `hybrid-failover.settings.main_section` (default `glob`) and logs a warning.

`/config_set` and the LuCI page can change `token`, `router_name`, `admin_ids`, `viewer_ids`, `policy`, `clash_api`, `routing_init_script`, `log_path`, `audit_path`, `probe_timeout_seconds`, `notify_failover_enabled` and `notify_failover_interval_seconds`. ID lists are separated by commas or spaces. Changes go to `/etc/hybrid-failover-bot.json.pending` first. `/config_apply` validates the pending file and writes it over the main one. The bot reads its config only at startup, so restart it afterwards:

```sh
/etc/init.d/hybrid-failover-bot restart
```

`routers`, `uci_package`, `main_section`, `allow_shell_ids` and the `watchdog_*` keys can only be changed in the JSON file itself.

The UCI section `hybrid-failover-bot.main` holds `enabled` (`0` after installation), `binary`, `config_path` and `log_path`. The init script passes the last one in `HF_BOT_LOG_PATH`, but the bot does not read that variable; the log is set by `log_path` in the JSON.

The core can send failover alerts without the bot, through the UCI option `hybrid-failover.settings.webhook_url`. See [docs/en/OVERVIEW.md](../docs/en/OVERVIEW.md#switch-notifications).

### Failover notifications

With `notify_failover_enabled: true` the bot reads `/var/log/hybrid-failover/history.jsonl` and sends new events to every `admin_ids` user. Users in `viewer_ids` do not get them. The message header carries the router name. The bot remembers the time of the last event it sent, so when the core rotates the file, events are neither skipped nor resent in bulk.

### Monitoring and self-repair

Every `watchdog_interval_seconds` (30 s by default) the bot asks each configured router for core RPC `Status` and looks at four things: is the engine running, are the nft rules in place, does FakeIP DNS answer, and is at least one channel alive. This is an outer layer on top of the core's own watchdog. The core repairs what it can see from inside, but it cannot message you and does not notice when it is stuck itself.

What the bot does:

1. One failed check means nothing. A reload after "Apply" takes a few seconds and is not an incident. The bot acts after three failures in a row, about a minute and a half.
2. If `watchdog_auto_repair` is on and the problem is one a restart can fix (engine, nft, FakeIP, a silent core), the bot tells the admins "trying to repair" and restarts `/etc/init.d/hybrid-failover`. If that has not helped a minute later, it stops the service and starts it again. It never makes more than two attempts per incident.
3. When the attempts are used up, or repair is off, a 🚨 message arrives with the problem, what the bot did, and the last log lines with secrets removed. While the incident lasts, the reminder repeats every 30 minutes.
4. If every channel is down, a restart will not help. The bot says so right away and touches nothing.
5. When everything is back, a ✅ message tells how long the incident lasted and what fixed it.

Only `admin_ids` get these messages. The `/watch` screen (the 🛡 Monitoring button) shows the state of every router, the open incident and the last closed one. Its buttons are "Check now", "Repair now" (a restart with confirmation, same as `/repair`), "Mute for 1 h" and "Unmute". `/mute <minutes>` silences the notices for 1 to 1440 minutes while repairs keep running, `/unmute` turns them back on.

If the bot itself or the whole router goes down, nobody is left to write: procd restarts the bot, and only a bot on another device can report a router that is completely off.

The same watchdog keeps an eye on subscriptions: when one ends within three days, has ended or failed to refresh, the admins get a message with a button to refresh the subscriptions. The "Renew or ask" button (the subscription's `Support-Url`, else `partner_url`) appears only when the subscription itself is the problem: it is ending, has ended or the panel refused it. When a refresh failed on the network and the subscription is active, the message says the servers from the last refresh keep working. While the situation lasts, the reminder repeats once a day.

### Updates

Once a day (`update_check_interval_hours`) the bot asks core whether a new release is out (`rpc UpdateCheck`). When one is, the admins get a message with the version, the start of the release notes, an "⬆️ Update now" button and a link to the notes. Each version is offered once. The button asks for confirmation, then core installs the packages and restarts the service and the bot; after the restart the new bot reports how it went. The same by hand: `/version` shows the installed and the available version, `/upgrade` updates, `/upgrade_status` shows the progress and the outcome of the last update. The "Version and update" screen is under "System". `update_check_enabled: false` turns the automatic check off, the commands stay.

### Access

Admins in `admin_ids` can do everything. Users in `viewer_ids` can only run `/start`, `/help`, `/panel`, `/quick`, `/wizard`, `/status`, `/health`, `/channels`, `/routes`, `/history`, `/failover_history`, `/failover_list`, `/uci_show`, `/uci_sections`, `/params`, `/param_list`, `/logs`, `/check_channels`, `/clients`, `/version`, `/upgrade_status`, the screens `/menu`, `/subs`, `/lists`, `/settings`, `/system`, `/manage`, `/watch`, the read-only router state commands `/sysinfo`, `/wan`, `/devices`, `/wifi`, `/syslog`, `/dmesg`, `/services`, `/portfwd`, `/slots`, and the router selection commands `/routers`, `/use` and `/router`. The selection only affects what that user sees. In the panel they can move between sections and press the buttons for those same commands. Buttons that ask for a value or a confirmation are closed to them. Anyone else gets a refusal. Every command goes to the audit log.


## Several routers

Telegram delivers the updates for one token to one consumer only. If the same token is configured in bots on two routers, they take turns grabbing commands, and each command runs on whichever router got it. There are two setups that work.

### One bot per router

Good when the routers are independent.

1. Create one bot per router in [@BotFather](https://t.me/BotFather), for example `home_hf_bot` and `office_hf_bot`.
2. Install the bot package on each router and put that router's own `token` into `/etc/hybrid-failover-bot.json`.
3. Leave `routers` out. The bot manages the router it runs on.
4. Set `router_name`, or leave it empty to use the hostname. The name shows up in `/panel`, `/status` and notifications, so the chat list makes it obvious which router is talking.

The bot notices a shared token by itself. When getUpdates returns HTTP 409, it warns the admins once an hour and logs `telegram token is polled by another bot instance`.

### One bot for all routers over SSH

Good when you want one chat for every router.

Install the bot on one host that can reach the other routers. On the others the bot stays off (`uci set hybrid-failover-bot.main.enabled=0`) and only the core is needed. A full config example is in [examples/hybrid-failover-bot.multi-router.json](../examples/hybrid-failover-bot.multi-router.json).

```json
"routers": [
  { "id": "home", "name": "Home", "local": true },
  {
    "id": "office",
    "name": "Office",
    "host": "192.168.11.1",
    "user": "root",
    "identity_file": "/etc/hybrid-failover-bot/id_office"
  }
]
```

- `local: true` marks the router the bot runs on. Commands go to the local `hybrid-failover`.
- A remote router needs `host` and `identity_file`. `port` defaults to 22 and `user` to `root`. The bot runs the same commands over SSH with that key. The public key goes into `/etc/dropbear/authorized_keys` on the router. The host key is not verified.
- Each router can override `uci_package`, `main_section`, `routing_init_script` and `clash_api`. Otherwise the top-level values apply.
- A router whose key file is missing on disk is skipped at startup with a warning in the log.
- `/routers` lists the routers, `/use office` selects one, `/router` shows the current one. Each admin has their own selection. It is kept in memory, so after a bot restart it has to be made again. Until a router is selected, router commands do not run.
- Replies start with `[Office]`, and `/panel` gets router selection buttons.
- Status, channels and history come from core RPC on the selected router, so they work over SSH too.
- Failover notifications only come from the router the bot runs on.

## Commands

`/help` prints the full list for the current section.

**General**

- `/menu` (also `/start` and `/panel`) opens the main menu: a summary of the engine, the active channel, how many channels are alive, monitoring and unapplied changes, with the section buttons below.
- The sections open as commands too: `/channels` channels, `/subs` subscriptions, `/lists` service lists and their channels, `/settings` failover settings, `/system` status, logs and maintenance, `/manage` the router, `/watch` monitoring. The same commands are registered in Telegram's "/" menu.
- Every screen is built from live router data. After an action the bot redraws the same screen with the result as a line on top. UCI changes pile up in pending, and the screens show their count with "Apply" and "Roll back" buttons.
- `/help` prints the full command list.
- `/quick` (or `/wizard`) shows common recipes.
- `/param_menu` shows parameter hints with quick setting buttons, `/uci_menu` shows hints for the UCI commands.
- `/cancel` drops a pending value prompt.

**Routers** (when there is more than one)

- `/routers` lists them.
- `/use <id>` selects a router.
- `/router` shows the selected one.

**State**

- `/status` shows the service state and active channels.
- `/health` (or `/check_channels`) probes the channels again through RPC Health.
- `/channels` (or `/failover_list`) shows the channels and whether they are up.
- `/routes` shows the section channels with numbers and where each service list goes. The `/lists` screen does the same with buttons: tapping a list opens the channel choice (pool, balance, direct, block or a specific channel).
- `/route <list> <channel> [pool|direct|block]` binds a list to a channel. The channel is a number from `/routes`, an id, the start of its name, or one of `pool`, `balance`, `direct`, `block`. The last argument says where to go when the channel is down. Example: `/route youtube 2`. The change goes to pending and is applied with `/param_apply`.
- `/history` (or `/failover_history`) prints the last 20 failover events.
- `/clients` shows the client rules.
- `/logs [N]` prints the last `logread` lines for hybrid-failover, 50 by default, 500 at most.

**Control**

- `/routing_restart` restarts `/etc/init.d/hybrid-failover`.
- `/switch <outbound>` or `/switch <section> <outbound>` switches the channel by hand.
- `/list_update` refreshes the domain lists.
- `/subscription_refresh` refreshes subscriptions.

**Subscriptions**

The `/subs` screen shows the `settings.subscription_urls` list and the `settings.subscription_update_interval` interval. Links are shown without their tail, because the tail is a secret token.

- The "Add" button waits for a link in one message, `/sub_add <https://…>` does the same. Only `http` and `https` without spaces or quotes are accepted, and a duplicate is not added.
- `/sub_del <number>` removes a subscription by its number in the list, with confirmation.
- `/sub_interval <1h|6h|12h|24h>` sets the refresh interval, values like `30m`, `6h`, `1d` are accepted.

Adding, removing and the interval go to pending and are applied with the "Apply" button or `/param_apply`. The "Refresh now" button runs the refresh right away.

Under every link the screen shows what the panel reported at the last refresh: its title, the number of servers, the expiry with days left, and the reason when it failed (for example that the subscription is not active). When the panel sends a `Support-Url`, a button to renew or ask appears. Servers from a subscription get the names the panel gave them as channel names. With no subscriptions the screen says where a subscription for the router can be obtained (`settings.partner_url` in the core UCI, `off` hides it).

**Monitoring**

- `/watch` opens the monitoring screen, see "Monitoring and self-repair".
- `/repair` restarts the service on the selected router, with confirmation.
- `/mute [minutes]` silences monitoring notices for that long (60 minutes by default, 1440 at most), `/unmute` turns them back on.

**Updates**

- `/version` shows the installed version and checks for a new one.
- `/upgrade [router]` updates Hybrid Failover to the latest release, with confirmation. The router is named when there are several; the button in the notice fills it in.
- `/upgrade_status` shows the progress and the outcome of the last update.

**Failover**

- `/failover_params` shows the policy and URLTest settings of the main section.
- `/failover_help` lists the failover editing commands.
- `/failover_add <uri>` adds a backup URI to `failover_proxy_links`, `/failover_rm <uri>` removes one.
- `/failover_apply` applies the changes.
- `/set_policy outage-only|prefer-primary|fastest` sets `failover_policy`.
- `/set_urltest_interval <sec>`, `/set_urltest_tolerance <ms>`, `/set_urltest_idle_timeout <sec>` and `/set_interrupt_existing on|off` set the URLTest options.
- `/set_quic on|off` turns QUIC on and off. `off` writes `settings.disable_quic=1`, `on` writes `0`.

**UCI parameters**

- `/params` (or `/param_list`) prints `uci show hybrid-failover`.
- `/param_get <key>`, `/param_set <key> <value>` and `/param_del <key>` read and change an option. The key can be a full `hybrid-failover.section.option` or an alias: `disable_quic`, `urltest_interval`, `urltest_check_interval`, `urltest_tolerance`, `urltest_idle_timeout`, `urltest_interrupt_exist_connections`, `policy`.
  For example `/param_set urltest_tolerance 100` or `/param_get disable_quic`.
- `/uci_show [section]` and `/uci_sections` print the config and the list of sections.
- `/uci_get`, `/uci_set` and `/uci_del` do the same as `/param_get`, `/param_set` and `/param_del`.
- `/uci_add_list <key> <value>` and `/uci_del_list <key> <value>` add and remove a list item.
- `/param_preview` shows the pending changes (`uci changes`).
- `/param_apply` validates and applies the pending changes. The core reloads the engine itself, no service restart is needed.
- `/param_rollback` discards the pending changes.

**Router management**

The menu has a "🖥 Роутер" (Router) button, or the `/manage` command, and everything below can be done with buttons. State, internet, Wi-Fi, services and port forwards open as screens, and every device, service and forward rule has its own card with actions (drop from Wi-Fi, wake, start and stop a service, delete a rule). The commands still work typed. They act on the router itself, not on Hybrid Failover. They use the same channel (local or SSH), so they work on every router in the `routers` list. Replies start with the router name when there are several.

- `/sysinfo` shows the model, system version, uptime, load, memory, space in `/overlay` and temperature.
- `/wan` shows the interfaces with address, gateway, DNS and link speed. `/ifup <name>` and `/ifdown <name>` bring an interface up or down.
- `/devices` lists the devices from the DHCP leases, with signal strength for those on Wi-Fi. `/wol <mac|name>` wakes a device (needs the `etherwake` or `wol` package). `/kick <mac|name>` drops a client from Wi-Fi for ten seconds.
- `/wifi` shows the radios, channels and client counts. `/wifi_on [radio]` and `/wifi_off [radio]` switch radios on and off, all of them without a name. The state survives a reboot.
- `/services` lists the services in `/etc/init.d`. `/service <name> start|stop|restart|reload|enable|disable` controls one. The bot itself cannot be stopped or restarted this way.
- `/portfwd` shows port forwards. `/portfwd_add <tcp|udp|tcpudp> <port> <ip> [dest_port] [name]` adds a rule (a port can be a range such as `8000-8010`), `/portfwd_del <number|name>` removes one. Both reload the firewall. A port that is already forwarded is not added twice.
- `/fw_restart` restarts the firewall.
- `/syslog [N]` and `/dmesg [N]` show the last lines of the system and kernel logs, 50 by default and 500 at most. Passwords, keys and the bot token are hidden in the output.
- `/ping <host>` and `/traceroute <host>` test connectivity from the router.
- `/apk_check` looks for package updates, `/apk_upgrade` upgrades all packages.
- `/backup` sends the configuration archive (`sysupgrade -b`) to the chat as a file. It holds Wi-Fi passwords, keys and the bot token, so do not forward it.
- `/reboot` reboots the router.

Beam WRT only. On other routers these commands answer that the tools are missing.

- `/update_check` and `/update_apply` check for and install a new firmware through `be7000-update`.
- `/slots` shows the firmware slots.
- `/mode5g [single|split|mlo]` shows and changes the 5 GHz mode.

**Arbitrary command**

`/sh <command>` runs a line on the router through `sh -c` as root. It is off by default. To turn it on, put your Telegram ID into `allow_shell_ids` in `/etc/hybrid-failover-bot.json` itself and restart the bot. The ID must also be in `admin_ids`. `/config_set` and the LuCI page cannot change this field, so a hijacked chat cannot switch the shell on by itself. Every run asks for confirmation and the whole command goes to the audit log. Think about whether you need it, because whoever gets hold of your Telegram account gets full access to the router.

**Bot config**

- `/config_show` shows `policy`, `clash_api`, `log_path` and `audit_path` from the pending or the main file.
- `/config_set <key> <value>` writes a key to the pending file.
- `/config_validate` validates the pending file and says whether it differs from the main one.
- `/config_apply` writes the pending file over the main one. Restart the bot afterwards.
- `/config_rollback` deletes the pending file.

Router commands that could cut you off, overwrite settings or send secrets out ask for confirmation both as a button and when typed. They are `/reboot`, `/ifdown`, `/wifi_off`, `/fw_restart`, `/portfwd_add`, `/portfwd_del`, `/update_apply`, `/apk_upgrade`, `/upgrade`, `/backup`, `/sh`, `/mode5g` with a mode, and `/service` with `stop`, `restart` or `disable`. The confirmation lasts 30 seconds, belongs to you and to the router that was selected, works once, and says what will happen. If you switched routers with `/use` in the meantime, it is cancelled.

The bot token and passwords are cut from the bot log, the audit log and the output of `/syslog`, `/dmesg` and `/sh`, because the Telegram client puts the token into its error text. Subscription and proxy links (`https://`, `vless://`, `ss://` and the rest) are shown in every bot reply without credentials or tail, `/uci_show` included.

UCI changes land in pending first, where `/param_preview` shows them before you apply or roll them back. The `/param_apply`, `/param_rollback`, `/failover_apply`, `/routing_restart`, `/config_apply`, `/config_rollback`, `/repair`, `/upgrade` buttons and subscription removal (`/sub_del`) ask for confirmation, which stays valid for 30 seconds.

## Timeouts and long replies

A command on the router gets 30 seconds by default. Slow operations get more time. A service restart may take up to 2 minutes, apply up to 3, a channel check up to 1, a list update up to 5 and a subscription refresh up to 3 minutes. Replies longer than Telegram's limit are split into several messages.

## LuCI

The **Services → Hybrid Failover → Telegram** page is at `/cgi-bin/luci/admin/services/hybrid-failover/bot`. It has fields for the same bot config keys as `/config_set`. Saving writes them to the pending file, and separate buttons validate, apply and roll back the pending file and restart the bot. Below that are the UCI settings of `hybrid-failover-bot`, that is, whether the service is enabled and the paths to the binary, config and log. `allow_shell_ids`, `watchdog_*`, `routers`, `uci_package` and `main_section` are not in the form, edit them in the JSON.

## Installing

The bot is part of the full install:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

On a router that already has the core, `HF_MODE=bot ash /tmp/install.sh` adds just the bot. The `hybrid-failover-bot` package depends on `hybrid-failover-core`. The LuCI **Telegram** tab appears once the bot is installed. After installing, fill in `token` and `admin_ids` and enable the bot:

```sh
uci set hybrid-failover-bot.main.enabled=1
uci commit hybrid-failover-bot
/etc/init.d/hybrid-failover-bot restart
```
