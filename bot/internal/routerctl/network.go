package routerctl

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ------------------------------------------------------------ interfaces

func (s Service) WAN(ctx context.Context) (string, error) {
	dump, err := s.ubus(ctx, "network.interface", "dump")
	if err != nil {
		return "", err
	}
	devs, _ := s.ubus(ctx, "network.device", "status")
	var lines []string
	for _, raw := range list(dump, "interface") {
		i, _ := raw.(map[string]any)
		name := str(i, "interface")
		if name == "" || name == "loopback" {
			continue
		}
		dev := str(i, "l3_device")
		if dev == "" {
			dev = str(i, "device")
		}
		mark := "✗"
		if flag(i, "up") {
			mark = "✓"
		}
		head := fmt.Sprintf("%s %s (%s)", mark, name, str(i, "proto"))
		if dev != "" {
			head += " " + dev
		}
		lines = append(lines, head)
		var details []string
		for _, a := range list(i, "ipv4-address") {
			am, _ := a.(map[string]any)
			details = append(details, fmt.Sprintf("адрес %s/%s", str(am, "address"), str(am, "mask")))
		}
		for _, a := range list(i, "ipv6-address") {
			am, _ := a.(map[string]any)
			details = append(details, fmt.Sprintf("адрес %s/%s", str(am, "address"), str(am, "mask")))
		}
		for _, r := range list(i, "route") {
			rm, _ := r.(map[string]any)
			if str(rm, "target") == "0.0.0.0" && str(rm, "mask") == "0" {
				details = append(details, "шлюз "+str(rm, "nexthop"))
				break
			}
		}
		if dns := list(i, "dns-server"); len(dns) > 0 {
			var d []string
			for _, x := range dns {
				if v, ok := x.(string); ok {
					d = append(d, v)
				}
			}
			details = append(details, "DNS "+strings.Join(d, ", "))
		}
		if flag(i, "up") {
			details = append(details, "аптайм "+fmtDuration(int64(num(i, "uptime"))))
		}
		if dm := sub(devs, dev); dm != nil {
			if sp := str(dm, "speed"); sp != "" && sp != "-" {
				details = append(details, "линк "+sp)
			}
		}
		for _, d := range details {
			lines = append(lines, "    "+d)
		}
	}
	if len(lines) == 0 {
		return "Интерфейсов нет.", nil
	}
	lines = append(lines, "", "Включить и выключить: /ifup <имя>, /ifdown <имя>")
	return strings.Join(lines, "\n"), nil
}

func (s Service) interfaceNames(ctx context.Context) ([]string, error) {
	dump, err := s.ubus(ctx, "network.interface", "dump")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, raw := range list(dump, "interface") {
		i, _ := raw.(map[string]any)
		if n := str(i, "interface"); n != "" && n != "loopback" {
			names = append(names, n)
		}
	}
	return names, nil
}

func (s Service) IfUpDown(ctx context.Context, name string, up bool) (string, error) {
	if !ValidIface(name) {
		return "", fmt.Errorf("недопустимое имя интерфейса")
	}
	names, err := s.interfaceNames(ctx)
	if err != nil {
		return "", err
	}
	ok := false
	for _, n := range names {
		if n == name {
			ok = true
		}
	}
	if !ok {
		return "", fmt.Errorf("интерфейса %q нет, список: /wan", name)
	}
	cmd, word := "ifdown", "выключен"
	if up {
		cmd, word = "ifup", "включён"
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	if _, err := s.r.Run(cctx, cmd, name); err != nil {
		return "", err
	}
	return fmt.Sprintf("Интерфейс %s %s.", name, word), nil
}

// --------------------------------------------------------------- devices

type lease struct {
	mac, ip, name string
}

func parseLeases(raw string) []lease {
	var out []lease
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		name := f[3]
		if name == "*" {
			name = ""
		}
		out = append(out, lease{mac: strings.ToLower(f[1]), ip: f[2], name: name})
	}
	return out
}

type station struct {
	mac    string
	iface  string
	signal float64
}

func (s Service) hostapdObjects(ctx context.Context) []string {
	out, err := s.r.Run(ctx, "ubus", "list")
	if err != nil {
		return nil
	}
	var objs []string
	for _, o := range strings.Fields(out) {
		if strings.HasPrefix(o, "hostapd.") {
			objs = append(objs, o)
		}
	}
	return objs
}

func (s Service) stations(ctx context.Context) []station {
	var out []station
	for _, obj := range s.hostapdObjects(ctx) {
		m, err := s.ubus(ctx, obj, "get_clients")
		if err != nil {
			continue
		}
		clients, _ := m["clients"].(map[string]any)
		for mac, v := range clients {
			cm, _ := v.(map[string]any)
			out = append(out, station{mac: strings.ToLower(mac), iface: strings.TrimPrefix(obj, "hostapd."), signal: num(cm, "signal")})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].mac < out[j].mac })
	return out
}

func (s Service) leases(ctx context.Context) []lease {
	raw, err := s.r.Run(ctx, "cat", "/tmp/dhcp.leases")
	if err != nil {
		return nil
	}
	return parseLeases(raw)
}

func (s Service) Devices(ctx context.Context) (string, error) {
	leases := s.leases(ctx)
	stas := s.stations(ctx)
	byMAC := map[string]station{}
	for _, st := range stas {
		byMAC[st.mac] = st
	}
	sort.Slice(leases, func(i, j int) bool { return ipLess(leases[i].ip, leases[j].ip) })
	seen := map[string]bool{}
	var lines []string
	for _, l := range leases {
		seen[l.mac] = true
		name := l.name
		if name == "" {
			name = "-"
		}
		line := fmt.Sprintf("%-15s %-22s %s", l.ip, name, l.mac)
		if st, ok := byMAC[l.mac]; ok {
			line += fmt.Sprintf("  Wi-Fi %s %.0f дБм", st.iface, st.signal)
		}
		lines = append(lines, line)
	}
	for _, st := range stas {
		if !seen[st.mac] {
			lines = append(lines, fmt.Sprintf("%-15s %-22s %s  Wi-Fi %s %.0f дБм", "-", "-", st.mac, st.iface, st.signal))
		}
	}
	if len(lines) == 0 {
		return "Устройств не найдено.", nil
	}
	head := fmt.Sprintf("Устройства (%d):", len(lines))
	return head + "\n" + strings.Join(lines, "\n"), nil
}

func ipLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	if len(pa) != 4 || len(pb) != 4 {
		return a < b
	}
	for i := 0; i < 4; i++ {
		var x, y int
		fmt.Sscanf(pa[i], "%d", &x)
		fmt.Sscanf(pb[i], "%d", &y)
		if x != y {
			return x < y
		}
	}
	return false
}

// resolveDevice turns a MAC address or a DHCP host name into a MAC address.
func (s Service) resolveDevice(ctx context.Context, who string) (string, error) {
	if mac, ok := NormalizeMAC(who); ok {
		return mac, nil
	}
	if !ValidName(who) {
		return "", fmt.Errorf("укажите MAC-адрес (aa:bb:cc:dd:ee:ff) или имя из /devices")
	}
	var hits []lease
	for _, l := range s.leases(ctx) {
		if strings.EqualFold(l.name, who) {
			hits = append(hits, l)
		}
	}
	switch len(hits) {
	case 0:
		return "", fmt.Errorf("устройство %q не найдено в /devices", who)
	case 1:
		return hits[0].mac, nil
	}
	return "", fmt.Errorf("имя %q встречается несколько раз, укажите MAC-адрес", who)
}

func (s Service) Kick(ctx context.Context, who string) (string, error) {
	mac, err := s.resolveDevice(ctx, who)
	if err != nil {
		return "", err
	}
	for _, obj := range s.hostapdObjects(ctx) {
		m, err := s.ubus(ctx, obj, "get_clients")
		if err != nil {
			continue
		}
		clients, _ := m["clients"].(map[string]any)
		for cm := range clients {
			if strings.EqualFold(cm, mac) {
				arg, _ := json.Marshal(map[string]any{"addr": mac, "deauth": true, "reason": 5, "ban_time": 10000})
				if _, err := s.r.Run(ctx, "ubus", "call", obj, "del_client", string(arg)); err != nil {
					return "", err
				}
				return fmt.Sprintf("%s отключён от Wi-Fi (%s), повторное подключение возможно через 10 секунд.", mac, strings.TrimPrefix(obj, "hostapd.")), nil
			}
		}
	}
	return "", fmt.Errorf("%s сейчас не подключён к Wi-Fi", mac)
}

func (s Service) WakeOnLAN(ctx context.Context, who string) (string, error) {
	mac, err := s.resolveDevice(ctx, who)
	if err != nil {
		return "", err
	}
	switch {
	case s.have(ctx, "/usr/bin/etherwake"):
		if _, err := s.r.Run(ctx, "/usr/bin/etherwake", "-i", "br-lan", mac); err != nil {
			return "", err
		}
	case s.have(ctx, "/usr/bin/wol"):
		if _, err := s.r.Run(ctx, "/usr/bin/wol", mac); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("на роутере нет etherwake или wol, установите: apk add etherwake")
	}
	return "Пакет пробуждения отправлен на " + mac + ".", nil
}

// ------------------------------------------------------------------ wifi

type radioInfo struct {
	name     string
	up       bool
	disabled bool
	band     string
	channel  string
	htmode   string
	ifaces   []string
	ssids    []string
}

func (s Service) radios(ctx context.Context) ([]radioInfo, error) {
	m, err := s.ubus(ctx, "network.wireless", "status")
	if err != nil {
		return nil, err
	}
	var out []radioInfo
	for name, v := range m {
		rm, _ := v.(map[string]any)
		cfg := sub(rm, "config")
		r := radioInfo{name: name, up: flag(rm, "up"), disabled: flag(rm, "disabled") || str(cfg, "disabled") == "1",
			band: str(cfg, "band"), channel: str(cfg, "channel"), htmode: str(cfg, "htmode")}
		for _, x := range list(rm, "interfaces") {
			im, _ := x.(map[string]any)
			ic := sub(im, "config")
			if ifn := str(im, "ifname"); ifn != "" {
				r.ifaces = append(r.ifaces, ifn)
			}
			if ss := str(ic, "ssid"); ss != "" {
				r.ssids = append(r.ssids, ss)
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

func (s Service) Wifi(ctx context.Context) (string, error) {
	radios, err := s.radios(ctx)
	if err != nil {
		return "", err
	}
	if len(radios) == 0 {
		return "Беспроводных радио нет.", nil
	}
	count := map[string]int{}
	for _, st := range s.stations(ctx) {
		count[st.iface]++
	}
	var lines []string
	for _, r := range radios {
		state := "выключено"
		switch {
		case r.up:
			state = "работает"
		case !r.disabled:
			state = "не поднялось"
		}
		head := fmt.Sprintf("%s: %s", r.name, state)
		if r.band != "" {
			head += ", " + r.band
		}
		if r.channel != "" {
			head += ", канал " + r.channel
		}
		if r.htmode != "" {
			head += " " + r.htmode
		}
		lines = append(lines, head)
		clients := 0
		for _, i := range r.ifaces {
			clients += count[i]
		}
		if len(r.ssids) > 0 {
			lines = append(lines, fmt.Sprintf("    сеть %s, клиентов %d", strings.Join(r.ssids, ", "), clients))
		}
	}
	lines = append(lines, "", "Включить и выключить: /wifi_on [радио], /wifi_off [радио]")
	return strings.Join(lines, "\n"), nil
}

func (s Service) WifiSet(ctx context.Context, radio string, on bool) (string, error) {
	radios, err := s.radios(ctx)
	if err != nil {
		return "", err
	}
	var targets []string
	for _, r := range radios {
		if radio == "" || r.name == radio {
			targets = append(targets, r.name)
		}
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("радио %q нет, список: /wifi", radio)
	}
	val := "1"
	word := "выключено"
	if on {
		val, word = "0", "включено"
	}
	for _, t := range targets {
		if !ValidRadio(t) {
			return "", fmt.Errorf("недопустимое имя радио %q", t)
		}
		if _, err := s.r.Run(ctx, "uci", "set", "wireless."+t+".disabled="+val); err != nil {
			return "", err
		}
	}
	if _, err := s.r.Run(ctx, "uci", "commit", "wireless"); err != nil {
		return "", err
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	if _, err := s.r.Run(cctx, "wifi", "reload"); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wi-Fi %s: %s.", word, strings.Join(targets, ", ")), nil
}
