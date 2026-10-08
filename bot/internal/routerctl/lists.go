package routerctl

import (
	"context"
	"sort"
	"strings"
)

// Structured views of the same data the text commands print, for screens that
// put a button next to every item.

type Iface struct {
	Name   string
	Proto  string
	Device string
	Up     bool
}

func (s Service) Interfaces(ctx context.Context) ([]Iface, error) {
	dump, err := s.ubus(ctx, "network.interface", "dump")
	if err != nil {
		return nil, err
	}
	var out []Iface
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
		out = append(out, Iface{Name: name, Proto: str(i, "proto"), Device: dev, Up: flag(i, "up")})
	}
	return out, nil
}

type Device struct {
	MAC    string
	IP     string
	Name   string
	WiFi   string // hostapd interface, empty for wired or offline
	Signal float64
}

func (s Service) DeviceList(ctx context.Context) ([]Device, error) {
	leases := s.leases(ctx)
	stas := s.stations(ctx)
	byMAC := map[string]station{}
	for _, st := range stas {
		byMAC[st.mac] = st
	}
	sort.Slice(leases, func(i, j int) bool { return ipLess(leases[i].ip, leases[j].ip) })
	seen := map[string]bool{}
	var out []Device
	for _, l := range leases {
		if seen[l.mac] {
			continue
		}
		seen[l.mac] = true
		d := Device{MAC: l.mac, IP: l.ip, Name: l.name}
		if st, ok := byMAC[l.mac]; ok {
			d.WiFi, d.Signal = st.iface, st.signal
		}
		out = append(out, d)
	}
	for _, st := range stas {
		if !seen[st.mac] {
			seen[st.mac] = true
			out = append(out, Device{MAC: st.mac, WiFi: st.iface, Signal: st.signal})
		}
	}
	return out, nil
}

type Radio struct {
	Name     string
	Up       bool
	Disabled bool
	Band     string
}

func (s Service) Radios(ctx context.Context) ([]Radio, error) {
	rs, err := s.radios(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Radio, 0, len(rs))
	for _, r := range rs {
		out = append(out, Radio{Name: r.name, Up: r.up, Disabled: r.disabled, Band: r.band})
	}
	return out, nil
}

type Svc struct {
	Name    string
	Running bool
	Enabled bool
}

func (s Service) ServiceList(ctx context.Context) ([]Svc, error) {
	names, err := s.initScripts(ctx)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	if out, err := s.r.Run(ctx, "ls", "/etc/rc.d"); err == nil {
		for _, e := range strings.Fields(out) {
			if len(e) > 3 && e[0] == 'S' {
				enabled[strings.TrimLeft(e[1:], "0123456789")] = true
			}
		}
	}
	running := map[string]bool{}
	if m, err := s.ubus(ctx, "service", "list"); err == nil {
		for k := range m {
			running[k] = true
		}
	}
	out := make([]Svc, 0, len(names))
	for _, n := range names {
		out = append(out, Svc{Name: n, Running: running[n], Enabled: enabled[n]})
	}
	return out, nil
}

// SelfService is the init script of the bot: it cannot be stopped from the chat.
func SelfService(name string) bool { return name == selfService }

type FwRule struct {
	Num      int // 1-based, as /portfwd shows it
	Name     string
	Proto    string
	SrcPort  string
	DestIP   string
	DestPort string
	Off      bool
	// Ref is what to give /portfwd_del: the name when it is unique and valid,
	// else the number (which shifts when rules change).
	Ref string
}

func (s Service) PortFwdRules(ctx context.Context) ([]FwRule, error) {
	rs, err := s.redirects(ctx)
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, r := range rs {
		count[r.name]++
	}
	out := make([]FwRule, 0, len(rs))
	for i, r := range rs {
		ref := ""
		if r.name != "" && ValidName(r.name) && count[r.name] == 1 {
			ref = r.name
		} else {
			ref = itoa(i + 1)
		}
		dp := r.destPrt
		if dp == "" {
			dp = r.srcPort
		}
		out = append(out, FwRule{Num: i + 1, Name: r.name, Proto: r.proto, SrcPort: r.srcPort, DestIP: r.destIP, DestPort: dp, Off: r.off, Ref: ref})
	}
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
