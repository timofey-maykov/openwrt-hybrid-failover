[Русский](../LUCI.md)

# LuCI user guide

The **`luci-app-hybrid-failover`** web interface configures Hybrid Failover on the router without editing UCI by hand.

**Menu:** Services → Hybrid Failover  
**URL:** `http://ROUTER/cgi-bin/luci/admin/services/hybrid-failover`

It needs **`hybrid-failover-core`** installed. If it is missing or the service is stopped, the pages show "No data" or an error.

The interface is written in Russian. English and Simplified Chinese translations exist (package `luci-i18n-hybrid-failover`) and follow the LuCI language. This guide quotes button and field names as they appear with LuCI set to English.

How the pages work inside (rpcd, ubus, build) is described in [luci/README.en.md](../../luci/README.en.md).

---

## Getting started

1. Install the packages as described in [INSTALL.md](INSTALL.md). The `full` mode installs the core, LuCI and the bot. The install script runs `hybrid-failover migrate` itself and enables and starts the service.
2. Open **Services → Hybrid Failover**. It lands on the **Overview** tab. The status next to the title should read "All good", and the Engine, Traffic interception and DNS (fake-IP) cards should show "running" and "OK".
3. On the **Routing** tab, add channels (links or a subscription) and choose which services go through them.
4. If needed, add rules on the **Clients** tab (a console, a TV, a work laptop).

---

## Tabs

| Tab | What it is for |
|-----|----------------|
| **Overview** | Live status: engine, traffic interception, DNS, current channel, channels with latency, switch log, manual switch |
| **Routing** | Global settings and routing sections: how to connect, channels, channel checks, service lists, your own domains and subnets |
| **Services and channels** | Section channels with latency and load, a "list → channel → if the channel is down → now" table, your own lists, automatic spread |
| **Charts** | Live per-channel charts, refreshed every 2 seconds |
| **Diagnostics** | Full check, individual checks, settings backup |
| **Clients** | Rules for individual devices by IP |
| **Telegram** | Bot settings and autostart of its service |
| **Update** | Checks GitHub for a new version and installs the release from LuCI |

---

## Overview

This page only shows state. No settings change here. Data refreshes every 5 seconds. The header shows the time of the last refresh, a **Refresh** button and a routing section picker (only when there is more than one section).

Next to the title is the overall status.

| Status | When |
|--------|------|
| **All good** | The engine is running, the interception rules are in place, DNS is fine |
| **Warnings** | The main VPN is down and traffic runs over a backup, the DNS check failed, or the core reported errors |
| **Not working** | The engine is stopped or the traffic interception rules (nftables) are missing |
| **No data** | The router state could not be read |

The header also shows which channel carries traffic right now and the core version.

Under the header is a large status block ("Traffic goes through the tunnel", "Working with warnings" or "The tunnel is not working") with errors, if any. Below it are four inner tabs.

**Summary.** Cards for Engine, Traffic interception, DNS (fake-IP), Now via, Channels up (for example `2 / 3`) and Mode with a hint on how it works. Then the section's channel cards with a **Check now** button, and the Route and Checks panels: mode, current channel, how long traffic has been on it, last switch, main VPN state and latency, channel check interval and threshold. The collapsed **Technical details** block holds the controller table with fail and success counters. On the right is the **Switch manually** block.

**Channels.** A channel table with state, channel, latency, trend and role (Main VPN, Backup, Channel, In use now). The trend is drawn from the latency history kept on the router. How many points it keeps is set by "Latency chart points" on the Routing tab. When the router has no history, the chart uses the last 40 samples the browser stored locally.

**Switches.** Recent switches of the selected section: time, from, to, reason and latency. Reasons are in plain words, for example "main VPN stopped responding". It shows the last 15 or 50 events.

**Tools.** Buttons **Check channels now**, **Full check**, **Check DNS**, **Export history** (downloads `failover-history.json`) and **Reset charts**. Reset charts clears only the latency history kept in the browser. The history on the router stays. The **Switch manually** block is repeated here.

**Check channels now** runs only when you press it. It re-checks every channel, which is a noticeable load on small routers, so the auto refresh does not do it. Without it, latencies come from the last background check.

### AWG channels

For a main channel on WireGuard or AmneziaWG the handshake is the main signal and the HTTP check is secondary. With a fresh handshake and a failed HTTP check the channel counts as up with the note `wireguard handshake OK`. With no handshake, or one older than 3 minutes, the channel is down with the reason `no wireguard handshake` or `wireguard handshake stale (...)`. These reasons come from the core in English.

### Manual switch

In the **Switch manually** block pick a section and a channel and press **Switch**. After you confirm, the section is pinned to that channel. In "Fastest channel" mode you cannot switch manually, the channel is picked automatically.

If the section runs a "Several links, the fastest one is used" group, the HTTP check picks the group member. An AWG card can show "up" by handshake while traffic still goes over a backup as long as the section is on the group. The error "switch request timed out" on old versions is fixed by updating the core to 1.7.35 or newer.

If the page shows an error or "No data", check `/etc/init.d/hybrid-failover status` and run **Full check** on the **Diagnostics** tab.

---

## Routing

This is where you set where traffic matched by Hybrid Failover goes: channels, service lists, your own domains and subnets.

The page header shows whether the engine is running and which channel carries traffic, and has two buttons.

- **Save and apply** writes the form changes and applies them. If the connection to the router is lost after applying, LuCI restores the previous settings by itself. With no changes, the button checks the saved config and applies it again. The result appears under the header.
- **More ▾** opens a menu:
  - **Check the saved config** checks the config without applying anything.
  - **Update service lists** downloads the service lists again.
  - **Update subscriptions** downloads the channels from subscriptions again.
  - **Copy section…** creates a copy of a section under a new name (Latin letters, no spaces). The copy is written right away and the page reloads.

### General settings

The block is split into tabs.

| Tab | What is there |
|-----|---------------|
| **Basic** | "Hybrid Failover enabled", "Disable QUIC", "Disable IPv6 on LAN", "Do not change dnsmasq and DHCP settings", "Bypass devices" |
| **DNS** | "DNS protocol", "DNS server", "Bootstrap DNS", "How to reach the upstream" (WAN and the active channel at once, WAN only, or the channel only), "DNS answer TTL, s" |
| **Subscriptions** | "Subscription links", the "Subscription status" block, "Update subscriptions" (by hand or on a schedule) |
| **Lists** | "Service list update interval", "Download lists through the tunnel" and through which section |
| **Clash API** | Clash API address, Yacd web panel, access from the internet, API secret |
| **Advanced** | "Main section", "Outgoing interface", "Main VPN check interval", "Webhook on switch", "Switch log lines", "Latency chart points", "Cache file" |

**Subscription status** shows what the panel reported at the last refresh: title, number of servers, expiry with days left, the reason of a failure. If the panel sent a link, **Renew or ask** appears next to it. With no subscriptions the block says where a subscription for the router can be obtained. Channels from a subscription appear in the channel list of the main section under their own names. A refresh replaces only the subscription channels; channels added by hand stay.

The Clash API tab, "Cache file" and "Outgoing interface" are left over from the old sing-box engine; the native engine does not use them. The "Hybrid Failover enabled" flag has no effect either, the service is enabled with `/etc/init.d/hybrid-failover enable`. Details in [UCI.md](UCI.md).

"Bypass devices" and "Devices fully through the tunnel" (in a section) are old lists. The core reads them only while the **Clients** tab has no rules. New rules are better set there.

### Routing sections

A section decides which traffic goes through which channels. One main section is usually enough. A new section is added with **Add section**, the name is free-form (`main`, `glob`, etc.). Section settings are split into tabs.

**Connection.** The main field is "Route traffic via".

| Value | Meaning |
|-------|---------|
| **Proxy channels** | Traffic goes through proxy links. "Channel setup": several links (the fastest one is used), a single link, or sing-box outbound JSON |
| **VPN interface** | Traffic goes through a VPN interface on the router (field "VPN interface", e.g. `awg0`). With "Backup channels when the VPN is down", backup proxies are added to it |
| **Block** | The section's domains and subnets are blocked |

When you switch between "Proxy channels" and "VPN interface", the channels already added move to the list of the new mode.

"When to switch" (for a VPN with backups):

| Value | How it works |
|-------|--------------|
| **Only when the VPN is down** | Traffic goes through the VPN while it answers. If the VPN is down, a backup channel takes over |
| **Return to the VPN as soon as it is back** | The same, but as soon as the VPN answers again, traffic moves back to it right away |
| **Always use the fastest channel** | There is no main channel: the VPN and the proxies compete on equal terms, traffic goes through the fastest live one. Channels do not show the "Backup" role |

The same tab has "Failed checks before switching", "Successful checks before returning" and the flag "Если все каналы недоступны, пускать напрямую" (if all channels are down, go direct; this label is not translated yet). With this flag, while no channel passes its check, the section's list traffic goes through the ISP instead of being cut off.

**Channels.** The "Channels" list (for the several-links mode) or "Backup channels" (for a VPN). Paste a link into the field at the bottom and press **Add**; several links can be pasted at once, one per line. vless, hysteria2, awg2, trojan, ss, socks and vpn:// work. Each channel shows its latency from the last check. Reorder by dragging or with the ↑ and ↓ arrows (tooltips "Move up" and "Move down"), ✕ deletes a channel. For backup channels, the higher a channel is, the earlier it is used. The pencil ✎ opens the link: you can edit it (**Save link**), see how the core reads it (**Decode**), or **Copy** it. The "UDP over TCP for Shadowsocks and SOCKS" flag is here too.

**Channel checks.** "Check interval", "Switch threshold, ms" (the channel changes only if another one is faster by at least that much), "Check URL", "Pause checks when idle", "Drop connections on channel switch".

**What to route.** "Service lists" (picked with chip buttons), "Own domains", "Own IPs and subnets" (as a list or as text) and "Devices fully through the tunnel".

**External lists.** "Domain lists by URL", "Subnet lists by URL", "Domain files on the router", "Subnet files on the router".

**Advanced.** "Own DNS for section servers" (for a VPN), with the protocol and address of that DNS. The native engine does not use these fields.

All UCI options are described in [UCI.md](UCI.md).

---

## Services and channels

![The services and channels tab](../img/luci-services.png)

At the top are the channel cards of each section: check state, latency, open connections, traffic and the share of expected load from the lists bound to it. The pencil next to the name renames a channel; the name is stored in `channel_names` and tied to the server, not to the position of the link. A rename is applied right away.

Below is the section's list table. Each list gets a channel: the pool (fastest, same as no binding), a specific channel, "Share by channels", direct or block. The "If the channel is down" column sets the fallback path: the section pool, direct, or block. The "Now" column shows where the list goes right now, for example that the channel is down and the list uses the pool.

"Share by channels" pins each site (second-level domain or IP) to one of the section's live channels, so a site does not change its external IP mid-session. Channels up to 1.5 times slower than the best get a full share of sites, up to 2.5 times a half share, slower ones get no new sites while others are available. Video CDNs (googlevideo.com, cdninstagram.com, fbcdn.net, telesco.pe, ttvnw.net, nflxvideo.net) are split by full server name, so video streams spread across channels while one video goes through one channel.

**Distribute automatically** puts lists on live channels: heavy services (video) land on different channels, faster channels get more. It needs at least two live channels. **Everything is through a pool** removes all bindings. You can adjust the result before saving. **+ Your list** creates a `user_list` with domains and subnets and lets you pick its channel right away.

Each section has its own "Save and apply" button; changes go through the usual LuCI apply with rollback. While changes are unsaved, "There are unsaved changes" is shown next to it.

## Charts

![The charts tab](../img/luci-charts.png)

Live charts per channel: inbound and outbound traffic, open and new connections, probe latency with 300 ms and 1 s thresholds and marks for failures, and the traffic share over the period. The window goes from 5 minutes to a day, refresh from 2 to 30 seconds, or paused.

Drag over a chart to zoom in, double-click to reset. Click a channel in a legend or a tile to show only that channel on every chart, Ctrl/Cmd+click hides or shows it. Hovering shows the values of all channels at that moment on every panel at once.

The native engine writes the data every 2 seconds (last 10 minutes) and every minute (a day) into router memory, so history starts over after a reboot. Bytes of interface channels (VPN, AWG) come from the interface counters; for proxy channels the engine counts them.

With a dark LuCI theme the charts are dark too:

![Charts in the dark theme](../img/luci-charts-dark.png)

---

## Diagnostics

Checks do not change any settings. The result appears in the **Result** block as a list of items, with the collapsed **Full response** holding the raw core answer below it.

| Button | What it checks |
|--------|----------------|
| **Full check** (in the header) | Engine, traffic interception, DNS and config at once. Takes up to a minute |
| **Individual checks → Config** | Builds the config from the settings and checks that the engine will accept it |
| **Individual checks → Traffic interception** | That the nftables rules sending traffic into the engine are in place |
| **Individual checks → DNS (fake-IP)** | That domains from the lists get service addresses and will go into the tunnel |

The **Settings backup** block works with a copy of all Hybrid Failover settings.

- **Download backup** sends the settings archive to the browser.
- **Save on the router** puts the archive into `/tmp/hybrid-failover-uci-backup.tar.gz`.
- **Restore from backup** (collapsed) replaces the current settings with those from a file on the router. The file must be in `/tmp`. After restoring, press **Save and apply** on the **Routing** tab so the engine rereads the settings.

Useful before experiments, when moving to another router, or when nothing opens from the LAN all of a sudden.

---

## Clients

This tab sets rules for individual home network devices by IP. For example, a console can go fully through the tunnel while a work laptop stays direct. Devices without a rule follow the section rules.

### Modes

| Mode | What happens |
|------|--------------|
| **All traffic through the tunnel** | Everything the device does goes through the chosen section. Good for consoles and TVs |
| **All traffic through Hybrid Failover, then by lists** | Only services from the lists go through the tunnel, but even direct access by IP is checked |
| **Leave alone (DNS and traffic bypass)** | Hybrid Failover leaves the device alone: its DNS is not replaced and its traffic does not enter the tunnel |
| **Always direct** | Even services from the lists go direct from this device, bypassing the tunnel |

This is how the page describes the modes. In practice the native engine has two caveats, see [UCI.md](UCI.md#mode-values):

- "Always direct" (`global_exclude`) is not handled by the native engine yet.
- "Leave alone" (`exclude`) only stops intercepting DNS the device sends to other servers. If the device asks the router for DNS, listed services get fake-IP addresses and still go through the tunnel. To keep a device out of the tunnel entirely, set an external DNS server on the device itself.

### Blocks on the page

| Block | What it is |
|-------|------------|
| **Rules** | The rule table: IP or subnet, device name (from DHCP leases), action, section (only for "All traffic through the tunnel"). Rows can be dragged |
| **In effect now** | Read-only: the rules as the running engine sees them. Columns Device, Action, Section |

If **In effect now** says "No device rules", there are no rules yet and every device follows the section rules. "The engine is not running" means the rules take effect once the service starts (`/etc/init.d/hybrid-failover start`).

If some rules come from the old "Bypass devices" and "Devices fully through the tunnel" fields on the Routing tab, the page says so. As soon as one rule exists here, the old fields are no longer used.

### Adding a rule

1. Press **Add a device from the network** in the header. In the dialog choose what to do with the device and, if needed, through which section. Then press **Add** next to the device. You can also do it by hand: **Add row** under the table, then type the IP.
2. Check the mode and section in the **Rules** table.
3. Press **Save & Apply** at the bottom of the page. This is the usual LuCI apply: if the connection to the router is lost, LuCI restores the previous settings. After applying, the service rereads the rules by itself.
4. Check the **In effect now** block. A row with the device and mode should appear.

### Add a device from the network

The device list comes from the router's DHCP leases. Devices that already have a rule are not shown.

If the list is empty:

- Check `/tmp/dhcp.leases` on the router.
- Make sure dnsmasq is running.
- Look at the standard LuCI page **Status → Overview**, where DHCP leases should also be listed.

---

## Telegram

This tab configures the bot (package `hybrid-failover-bot`). It is shown only when the bot is installed. The header shows the bot state ("Bot running" or "Bot stopped", "Autostart off") and two buttons: **Restart** and **Save bot settings**.

**Bot.** "Bot token" (issued by @BotFather, **Show** reveals it), "Router name" (how the router is called in notifications and the bot panel, empty means the hostname), "Administrators" and "View only" (Telegram IDs, comma-separated).

**Notifications.** "Report channel switches" and "At most once every, s" (10 or more). The collapsed **Advanced** block has the Clash API address (needed only by old sing-box installs), the Hybrid Failover service script, the check timeout, the bot log and the action log.

The `routers` list, `allow_shell_ids` and the `watchdog_*` keys are not in the form; edit them in `/etc/hybrid-failover-bot.json` by hand.

**Save bot settings** writes the changed fields, applies them and restarts the bot. If any step fails, the draft is rolled back, the bot's working settings stay as they were, and the reason is shown under the header.

**Service.** A standard LuCI form with the **Startup** tab ("Start the bot with the router") and the **Files** tab (bot program, settings file, log). It is saved with the LuCI buttons at the bottom of the page, separately from the header button.

Bot commands are described in [bot/README.en.md](../../bot/README.en.md).

---

## When changes take effect

| Page | What happens |
|------|--------------|
| **Routing** | The **Save and apply** button in the header. Changes are applied with rollback and the service rereads the settings. With no changes the button re-applies the saved config |
| **Services and channels** | The **Save and apply** button of each section. Renaming a channel and deleting your own list apply right away |
| **Clients** | **Save & Apply** at the bottom of the page, the service rereads the rules |
| **Telegram** | **Save bot settings** applies the settings and restarts the bot right away. Autostart is saved with the LuCI buttons at the bottom |
| **Diagnostics** | Restoring from a backup changes the settings file but does not apply it. Press **Save and apply** on the Routing tab |

---

## Update

**Check for updates** asks GitHub for the latest release. The **Versions** block shows the installed version, the latest one on GitHub and the time of the check. If GitHub has a newer version, an **Update to %s** button with the version number appears, along with a block listing the changes.

After you confirm, the update runs in the background. The router downloads the packages for its architecture from the release, checks each file against `manifest.json` by sha256 and size, installs them with `apk` or `opkg`, runs `migrate` and restarts rpcd and the services. The page follows the progress, shows the current step and log, and reloads when it is done. The connection to the router may drop for a few seconds. UCI settings are left alone. The bot and curfew are updated only if they are already installed.

The same from the console:

```sh
hybrid-failover update check     # what GitHub has
hybrid-failover update apply     # install the latest release (in the background)
hybrid-failover update status    # progress and log of the last update
```

`apply --foreground` installs in the current console, `--tag v1.7.74` picks a specific release, `--force` allows the same or an older version.

---

## Related docs

| Document | Contents |
|----------|----------|
| [OVERVIEW.md](OVERVIEW.md) | Architecture, URIs, DNS, failover |
| [UCI.md](UCI.md) | All UCI options, including `client_rule` |
| [INSTALL.md](INSTALL.md) | Installing the packages |
| [luci/README.en.md](../../luci/README.en.md) | LuCI sources, rpcd, package build |
