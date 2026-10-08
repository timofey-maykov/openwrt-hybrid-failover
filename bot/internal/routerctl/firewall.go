package routerctl

import (
	"context"
	"fmt"
	"strings"
)

// redirect is one DNAT section of /etc/config/firewall (a port forward).
type redirect struct {
	id      string
	name    string
	proto   string
	srcPort string
	destIP  string
	destPrt string
	target  string
	off     bool
}

// parseRedirects reads `uci show firewall` output.
func parseRedirects(show string) []redirect {
	var order []string
	byID := map[string]*redirect{}
	for _, line := range strings.Split(show, "\n") {
		line = strings.TrimSpace(line)
		eq := strings.Index(line, "=")
		if eq < 0 || !strings.HasPrefix(line, "firewall.") {
			continue
		}
		key, val := line[len("firewall."):eq], line[eq+1:]
		if !strings.Contains(key, ".") {
			if strings.Trim(val, "'") == "redirect" {
				order = append(order, key)
				byID[key] = &redirect{id: key}
			}
			continue
		}
		dot := strings.Index(key, ".")
		id, opt := key[:dot], key[dot+1:]
		r, ok := byID[id]
		if !ok {
			continue
		}
		v := strings.Join(strings.Fields(strings.NewReplacer("'", " ").Replace(val)), " ")
		switch opt {
		case "name":
			r.name = v
		case "proto":
			r.proto = v
		case "src_dport":
			r.srcPort = v
		case "dest_ip":
			r.destIP = v
		case "dest_port":
			r.destPrt = v
		case "target":
			r.target = v
		case "enabled":
			r.off = v == "0"
		}
	}
	var out []redirect
	for _, id := range order {
		r := byID[id]
		if strings.EqualFold(r.target, "SNAT") {
			continue
		}
		out = append(out, *r)
	}
	return out
}

func (s Service) redirects(ctx context.Context) ([]redirect, error) {
	out, err := s.r.Run(ctx, "uci", "show", "firewall")
	if err != nil {
		return nil, err
	}
	return parseRedirects(out), nil
}

func (r redirect) String(n int) string {
	name := r.name
	if name == "" {
		name = "-"
	}
	proto := r.proto
	if proto == "" {
		proto = "tcp udp"
	}
	dp := r.destPrt
	if dp == "" {
		dp = r.srcPort
	}
	state := ""
	if r.off {
		state = "  (выключен)"
	}
	return fmt.Sprintf("%d. %s  %s  порт %s → %s:%s%s", n, name, proto, r.srcPort, r.destIP, dp, state)
}

func (s Service) PortFwdList(ctx context.Context) (string, error) {
	rs, err := s.redirects(ctx)
	if err != nil {
		return "", err
	}
	if len(rs) == 0 {
		return "Проброса портов нет.\nДобавить: /portfwd_add <tcp|udp|tcpudp> <порт> <ip> [порт_назначения] [имя]", nil
	}
	lines := []string{"Проброс портов:"}
	for i, r := range rs {
		lines = append(lines, r.String(i+1))
	}
	lines = append(lines, "", "Добавить: /portfwd_add <tcp|udp|tcpudp> <порт> <ip> [порт_назначения] [имя]", "Удалить: /portfwd_del <номер|имя>")
	return strings.Join(lines, "\n"), nil
}

func (s Service) reloadFirewall(ctx context.Context) error {
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	_, err := s.r.Run(cctx, "/etc/init.d/firewall", "reload")
	return err
}

// PortFwdAdd: args are proto, external port, LAN address, optional internal
// port and name.
func (s Service) PortFwdAdd(ctx context.Context, proto, srcPort, ip, destPort, name string) (string, error) {
	switch proto {
	case "tcp", "udp", "tcpudp":
	default:
		return "", fmt.Errorf("протокол: tcp, udp или tcpudp")
	}
	sp, err := NormalizePort(srcPort)
	if err != nil {
		return "", err
	}
	dp := sp
	if destPort != "" {
		if dp, err = NormalizePort(destPort); err != nil {
			return "", err
		}
	}
	if !ValidIPv4(ip) {
		return "", fmt.Errorf("адрес назначения должен быть IPv4 из локальной сети")
	}
	if name == "" {
		name = "hfbot_" + strings.ReplaceAll(sp, "-", "_")
	}
	if !ValidName(name) {
		return "", fmt.Errorf("имя: латиница, цифры, точка, дефис и подчёркивание, до 32 символов")
	}
	existing, err := s.redirects(ctx)
	if err != nil {
		return "", err
	}
	for _, e := range existing {
		if e.srcPort == sp && (e.proto == proto || e.proto == "" || proto == "tcpudp" || (e.proto == "tcp udp")) {
			return "", fmt.Errorf("порт %s уже проброшен (%s → %s), сначала /portfwd_del", sp, e.name, e.destIP)
		}
	}
	id, err := s.r.Run(ctx, "uci", "add", "firewall", "redirect")
	if err != nil || !ValidService(id) {
		return "", fmt.Errorf("uci add: не удалось создать правило")
	}
	revert := func() { _, _ = s.r.Run(ctx, "uci", "revert", "firewall") }
	sets := [][]string{
		{"set", "firewall." + id + ".name=" + name},
		{"set", "firewall." + id + ".target=DNAT"},
		{"set", "firewall." + id + ".src=wan"},
		{"set", "firewall." + id + ".dest=lan"},
		{"set", "firewall." + id + ".src_dport=" + sp},
		{"set", "firewall." + id + ".dest_ip=" + ip},
		{"set", "firewall." + id + ".dest_port=" + dp},
	}
	if proto == "tcpudp" {
		sets = append(sets, []string{"add_list", "firewall." + id + ".proto=tcp"}, []string{"add_list", "firewall." + id + ".proto=udp"})
	} else {
		sets = append(sets, []string{"set", "firewall." + id + ".proto=" + proto})
	}
	for _, a := range sets {
		if _, err := s.r.Run(ctx, "uci", a...); err != nil {
			revert()
			return "", err
		}
	}
	if _, err := s.r.Run(ctx, "uci", "commit", "firewall"); err != nil {
		revert()
		return "", err
	}
	if err := s.reloadFirewall(ctx); err != nil {
		return "", fmt.Errorf("правило записано, но файрвол не перезагрузился: %v", shortErr(err))
	}
	return fmt.Sprintf("Проброс добавлен: %s порт %s → %s:%s (%s).", proto, sp, ip, dp, name), nil
}

// PortFwdDel removes a forward by its number in /portfwd or by name.
func (s Service) PortFwdDel(ctx context.Context, ref string) (string, error) {
	rs, err := s.redirects(ctx)
	if err != nil {
		return "", err
	}
	idx := -1
	var n int
	if _, err := fmt.Sscanf(ref, "%d", &n); err == nil && fmt.Sprint(n) == ref && n >= 1 && n <= len(rs) {
		idx = n - 1
	} else if ValidName(ref) {
		for i, r := range rs {
			if r.name == ref {
				if idx >= 0 {
					return "", fmt.Errorf("имя %q встречается несколько раз, используйте номер из /portfwd", ref)
				}
				idx = i
			}
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("правило %q не найдено, список: /portfwd", ref)
	}
	r := rs[idx]
	if !ValidService(r.id) && !strings.HasPrefix(r.id, "@redirect[") {
		return "", fmt.Errorf("неожиданный идентификатор правила")
	}
	if _, err := s.r.Run(ctx, "uci", "delete", "firewall."+r.id); err != nil {
		return "", err
	}
	if _, err := s.r.Run(ctx, "uci", "commit", "firewall"); err != nil {
		_, _ = s.r.Run(ctx, "uci", "revert", "firewall")
		return "", err
	}
	if err := s.reloadFirewall(ctx); err != nil {
		return "", fmt.Errorf("правило удалено, но файрвол не перезагрузился: %v", shortErr(err))
	}
	return "Удалено: " + r.String(idx+1), nil
}

func (s Service) FirewallRestart(ctx context.Context) (string, error) {
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	if _, err := s.r.Run(cctx, "/etc/init.d/firewall", "restart"); err != nil {
		return "", err
	}
	return "Файрвол перезапущен.", nil
}
