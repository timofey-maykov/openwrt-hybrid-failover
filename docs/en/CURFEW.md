[Русский](../CURFEW.md)

# Curfew

Curfew limits the speed of chosen LAN devices at night. At a set time it adds `tc` rules on the LAN interface (`br-lan` by default) and removes them in the morning. Each device gets its own limit, in both directions. Other devices are not limited.

Curfew is two packages: `curfew` (scripts, init script, UCI) and `luci-app-curfew` (the LuCI page). They do not depend on Hybrid Failover. Curfew does not read the `hybrid-failover` config and does not touch routing, nftables or the engine. The packages live in the same repository and ship in the same releases.

---

## Installation

`install-on-router.sh` does not install curfew in any `HF_MODE` (`full`, `core`, `bot`). Install the packages by hand.

Both packages are the same for all architectures (`all`). Download them from the [GitHub releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) page. File names for version 1.7.74:

| Package | OpenWrt 24.x (opkg) | OpenWrt 25.12+ (apk) |
|---------|---------------------|----------------------|
| `curfew` | `curfew_1.7.74-1_all.ipk` | `curfew-1.7.74-r1.apk` |
| `luci-app-curfew` | `luci-app-curfew_1.7.74-1_all.ipk` | `luci-app-curfew-1.7.74-r1.apk` |

### OpenWrt 24.x (opkg)

```sh
cd /tmp
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/curfew_1.7.74-1_all.ipk
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/luci-app-curfew_1.7.74-1_all.ipk
opkg update
opkg install /tmp/curfew_1.7.74-1_all.ipk
opkg install /tmp/luci-app-curfew_1.7.74-1_all.ipk
```

### OpenWrt 25.12+ (apk)

```sh
cd /tmp
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/curfew-1.7.74-r1.apk
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/luci-app-curfew-1.7.74-r1.apk
apk update
apk add --allow-untrusted /tmp/curfew-1.7.74-r1.apk
apk add --allow-untrusted /tmp/luci-app-curfew-1.7.74-r1.apk
```

The packages are not signed with the OpenWrt key, so `--allow-untrusted` is required.

### Dependencies

| Package | Depends on |
|---------|------------|
| `curfew` | `uci`, `tc`, `kmod-sched-core`, `kmod-ifb`, `kmod-sched-connmark`, `dnsmasq` |
| `luci-app-curfew` | `luci-base`, `luci-compat`, `curfew` |

The package manager installs dependencies from the OpenWrt feeds itself, so run `opkg update` or `apk update` first.

### What installation does

- `curfew` installs `/usr/sbin/curfew`, `/usr/sbin/curfew-apply`, `/usr/sbin/curfew-dhcp`, `/etc/init.d/curfew`, `/etc/config/curfew` and an empty `/etc/dnsmasq.d/curfew.conf`. `/etc/config/curfew` and `/etc/dnsmasq.d/curfew.conf` are conffiles and are not overwritten on upgrade.
- The `curfew` postinst runs `curfew-apply` and enables `/etc/init.d/curfew` at boot.
- The `luci-app-curfew` postinst restarts `rpcd` and `uhttpd` and clears the LuCI cache.

After installation curfew is off (`enabled '0'`).

### Updates

`hybrid-failover update apply` and the update button in the Hybrid Failover LuCI update `curfew` and `luci-app-curfew` only if they are already installed. If the packages are missing, the update does not install them.

### Build

`scripts/build-packages.sh` builds both packages together with the rest when the repository has the `curfew/root` and `luci-curfew/root` directories. `HF_BUILD_SET` does not affect them. Files come from `curfew/root` and `luci-curfew/root`, maintainer scripts from `packages/curfew/CONTROL` and `packages/luci-app-curfew/CONTROL`. The version is shared with Hybrid Failover (the `VERSION` file). The output goes to `dist/ipk`, `dist/apk` and `dist/manifest.json`.

---

## LuCI page

**Services → Curfew** (`admin/services/curfew`). The page UI is in Russian only. The English names below are translations.

**Настройки** (Settings) block:

| Field | UCI option |
|-------|------------|
| Включено (Enabled) | `enabled` |
| Начало лимита (Limit start) | `block_time` |
| Конец лимита (Limit end) | `unblock_time` |
| Скорость по умолчанию (Default rate) | `rate_limit` |
| Burst по умолчанию (Default burst) | `burst` |
| Интерфейс LAN (LAN interface) | `lan_if` |

**Устройства** (Devices) block, one row per device:

| Field | UCI option |
|-------|------------|
| Имя DHCP (DHCP name) | `name` |
| IP | `ip` |
| Скорость (Rate) | `rate_limit` |
| Burst | `burst` |

Empty **Скорость** and **Burst** mean the defaults. **IP** is a required field in LuCI. A device without an IP can only be set through `uci`.

The **Статус** (Status) block shows the limit state, the schedule window, the defaults and the device list with IP, rate, `online`/`offline` and the MAC from the lease. **Обновить** (Refresh) reloads the status. **Sync tc** runs `curfew sync`.

The **DHCP** block. **Выбрать из DHCP** (Pick from DHCP) shows the current leases from `/tmp/dhcp.leases`. **Выбрать** (Select) fills the lease name and IP into the device form.

**Save & Apply** saves UCI and runs `curfew-apply`. Then the message «Настройки применены (cron, dnsmasq, tc)» (settings applied) appears.

The page talks to the rpcd object `curfew` (ucode, `/usr/share/rpcd/ucode/curfew`):

| Method | What it does |
|--------|--------------|
| `status` | `curfew status-json`, returned as JSON |
| `apply` | `curfew-apply` |
| `sync` | `curfew sync` |
| `dhcp_leases` | Parses `/tmp/dhcp.leases` |

Access is set by the `luci-app-curfew` ACL: read `status` and `dhcp_leases`, write `apply` and `sync`, UCI `curfew`.

---

## UCI options

The configuration lives in **`/etc/config/curfew`**, and the UCI package is called **`curfew`**.

### `config curfew`

There is one anonymous section. The scripts read `curfew.@curfew[0]`.

The "Default" column shows the value from the `/etc/config/curfew` template. If the option is missing entirely, the scripts use a different value, given in the description.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | `0` / `1` | `0` | `1` turns the limit on. With `0` the `throttle` and `sync` commands add no rules |
| `block_time` | `HH:MM` | `01:30` | Limit start, in router time. If missing: `00:30` |
| `unblock_time` | `HH:MM` | `08:00` | Limit end. If missing: `08:00` |
| `lan_if` | interface | `br-lan` | Interface that gets the `tc` rules |
| `rate_limit` | `tc` rate | `128kbit` | Rate for devices without their own. `tc` format: `64kbit`, `128kbit`, `1mbit`. If missing: `64kbit` |
| `burst` | `tc` size | `16k` | Burst for devices without their own. If missing: `16k` |

### `config device`

One section per device. There can be any number of sections.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `name` | string | none | Device hostname from DHCP (the fourth field in `/tmp/dhcp.leases`). Required. A section without `name` and all sections after it are not read |
| `ip` | IPv4 | empty | Device address. If empty, the IP is taken from the lease in `/tmp/dhcp.leases` with the same hostname |
| `rate_limit` | `tc` rate | empty | Device's own rate. Empty: `rate_limit` from `config curfew` |
| `burst` | `tc` size | empty | Device's own burst. Empty: `burst` from `config curfew` |

Example:

```
config curfew
	option enabled '1'
	option block_time '23:00'
	option unblock_time '07:00'
	option lan_if 'br-lan'
	option rate_limit '128kbit'
	option burst '16k'

config device
	option name 'iPhone'
	option ip '192.168.1.123'
	option rate_limit '64kbit'
```

After editing with `uci`, apply the settings:

```sh
uci commit curfew
/etc/init.d/curfew reload
```

---

## Commands

| Command | What it does |
|---------|--------------|
| `curfew throttle` | Adds the `tc` rules if `enabled=1`. `curfew block` does the same |
| `curfew unblock` | Removes the `tc` rules |
| `curfew sync` | Adds the rules if `enabled=1` and the limit window is now. Otherwise removes them |
| `curfew status` | State, device list and schedule as text |
| `curfew status-json` | The same as JSON, for LuCI |
| `curfew-apply` | Writes cron and `/etc/dnsmasq.d/curfew.conf`, restarts cron, reloads dnsmasq, runs `curfew sync` |
| `/etc/init.d/curfew start` | `curfew-apply` |
| `/etc/init.d/curfew reload` | `curfew-apply` |
| `/etc/init.d/curfew stop` | `curfew unblock` |

---

## How it works

### Schedule

`curfew-apply` writes two lines to `/etc/crontabs/root` and restarts cron:

```
30 1 * * * /usr/sbin/curfew throttle
0 8 * * * /usr/sbin/curfew unblock
```

Old curfew lines are removed first, other cron jobs stay. The times come from `block_time` and `unblock_time` and use the router's time zone.

If the router boots or the settings are applied inside the window, `curfew sync` at the end of `curfew-apply` adds the rules. The window may cross midnight, for example `23:00`–`07:00`.

### dnsmasq

`curfew-apply` rewrites `/etc/dnsmasq.d/curfew.conf`:

- for each device with a known IP, a line `dhcp-host=<name>,<ip>,infinite`. The device with this hostname gets the same address with a lease that never expires;
- the line `dhcp-script=/usr/sbin/curfew-dhcp`.

Then dnsmasq reloads its configuration (`reload`, `restart` if that fails), if the dnsmasq service is enabled.

dnsmasq calls `curfew-dhcp` when it hands out or renews a lease (`add`, `old`). If the client hostname matches the `name` of one of the devices, it runs `curfew sync`. This updates the rules when a device gets its address.

### tc

The rules go on `lan_if`. For each device:

- **upload** (traffic from the device): an `ingress` qdisc and a `flower src_ip <ip>/32` filter with `police rate <rate> burst <burst>`. Anything over the limit is dropped;
- **download** (traffic to the device): a root `prio` qdisc, each device has its own band with `tbf rate <rate> burst <burst> latency 400ms` and a `flower dst_ip <ip>/32` filter. The last band is for all other traffic, with `tbf rate 1000mbit burst 64k latency 50ms`.

Before adding rules, the old root and ingress qdiscs on `lan_if` are deleted. After a successful setup the marker `/var/run/curfew.active` is created. `status` uses it to tell whether the limit is active. `unblock` deletes both qdiscs and the marker.

If at least one device has no IP (not set in UCI and no lease), no rules are added for any device.

---

## Checks

```sh
curfew status
uci show curfew
grep curfew /etc/crontabs/root
cat /etc/dnsmasq.d/curfew.conf
cat /tmp/dhcp.leases
tc qdisc show dev br-lan
tc filter show dev br-lan ingress
tc filter show dev br-lan parent 1:
tc -s qdisc show dev br-lan
ls -l /var/run/curfew.active
```

| Symptom | What to check |
|---------|---------------|
| The limit does not start at night | `enabled` is `1`. The curfew lines are in `/etc/crontabs/root` and cron is running (`/etc/init.d/cron status`). Router time and time zone (`date`) |
| `curfew status` shows `inactive` inside the window | Every device has an IP: `curfew status` must list all devices. Run `curfew sync` and look at `tc qdisc show dev br-lan` |
| A device is missing from the list | `name` matches the hostname in `/tmp/dhcp.leases`. Or set `ip` explicitly |
| `tc` errors about `flower`, `police`, `prio` | The kernel lacks the modules. Check `lsmod \| grep -E 'cls_flower\|act_police\|sch_prio\|sch_tbf\|sch_ingress'` |
| A device does not get the pinned IP | dnsmasq reads the `/etc/dnsmasq.d` directory. Look at `conf-dir` in the running dnsmasq config: `grep conf-dir /var/etc/dnsmasq.conf*` |
| The limit is not removed in the morning | The `curfew unblock` line is in cron. Remove it by hand: `curfew unblock` or `/etc/init.d/curfew stop` |
| The LuCI page is empty or the menu item is missing | Restart rpcd and clear the cache: `/etc/init.d/rpcd restart; rm -rf /tmp/luci-*cache*` |

---

## Limitations

- IPv4 only. The `tc` filters use `protocol ip`, a device's IPv6 traffic is not limited.
- A device is identified by its DHCP hostname. If the device changes its hostname or does not send one, set `ip` explicitly.
- If one device has no IP, no rules are added for any device.
- While the limit is active, curfew owns the root and ingress qdiscs on `lan_if`. Other qdiscs on this interface (SQM, for example) are deleted when the limit starts and when it is removed.
- While the limit is active, all other traffic to the LAN goes through the band with `tbf 1000mbit`.
- In the code, unfiltered traffic is sent to `prio` bands by `priomap 2 2 2 2 2 2 2 2 1 1 1 1 1 1 1 0`, and there are `number of devices + 1` bands. With one device in the list there are two bands and `tc` may reject this `priomap`: download is then not limited and `curfew status` shows `inactive`. With three or more devices, ordinary unfiltered traffic lands in the third device's band and is limited too. Exactly two devices work without issues. Check with `tc -s qdisc show dev br-lan`.
- The `tc` rules do not survive a reboot. At boot `/etc/init.d/curfew` (`START=99`) adds them through `curfew sync`.
- `/etc/init.d/curfew stop` removes the rules, but the cron lines stay. With `enabled=0` they add nothing.
- Removing the package does not delete the cron lines or `/etc/dnsmasq.d/curfew.conf`, remove them by hand.
- Hardware and software flow offloading may let part of the traffic bypass `tc`.
