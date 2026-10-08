// Package routerctl manages the router itself (system, network, Wi-Fi,
// firewall, updates) through the same executor the Hybrid Failover commands
// use, so every command works on the local router and on remote ones over SSH.
package routerctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerexec"
)

const (
	longTimeout = 10 * time.Minute
	midTimeout  = 90 * time.Second
	pingTimeout = 25 * time.Second
	// the init script of the bot itself: stopping it would cut the answer off
	selfService = "hybrid-failover-bot"
)

type Service struct {
	r routerexec.Exec
}

func New(r routerexec.Exec) Service { return Service{r: r} }

func (s Service) have(ctx context.Context, path string) bool {
	_, err := s.r.Run(ctx, "test", "-x", path)
	return err == nil
}

func (s Service) ubus(ctx context.Context, obj, method string, params ...string) (map[string]any, error) {
	args := append([]string{"call", obj, method}, params...)
	out, err := s.r.Run(ctx, "ubus", args...)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return nil, fmt.Errorf("ubus %s %s: не удалось разобрать ответ", obj, method)
	}
	return m, nil
}

func withBudget(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

// ---------------------------------------------------------------- system

func (s Service) SysInfo(ctx context.Context) (string, error) {
	board, err := s.ubus(ctx, "system", "board")
	if err != nil {
		return "", err
	}
	info, err := s.ubus(ctx, "system", "info")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Роутер: %s (%s)\n", str(board, "model"), str(board, "hostname"))
	fmt.Fprintf(&b, "Система: %s, ядро %s\n", str(sub(board, "release"), "description"), str(board, "kernel"))
	if v, err := s.r.Run(ctx, "sed", "-n", "s/^VERSION=//p", "/etc/be7000-release"); err == nil && v != "" {
		fmt.Fprintf(&b, "Сборка Beam WRT: %s\n", strings.Fields(v)[0])
	}
	fmt.Fprintf(&b, "Аптайм: %s\n", fmtDuration(int64(num(info, "uptime"))))
	if l := list(info, "load"); len(l) == 3 {
		f := func(i int) float64 { v, _ := l[i].(float64); return v / 65536 }
		fmt.Fprintf(&b, "Нагрузка: %.2f %.2f %.2f\n", f(0), f(1), f(2))
	}
	if mem := sub(info, "memory"); mem != nil {
		total, avail := num(mem, "total"), num(mem, "available")
		if avail == 0 {
			avail = num(mem, "free") + num(mem, "buffered") + num(mem, "cached")
		}
		fmt.Fprintf(&b, "Память: занято %s из %s (доступно %s)\n", fmtMB(total-avail), fmtMB(total), fmtMB(avail))
	}
	if out, err := s.r.Run(ctx, "df", "-k", "/overlay"); err == nil {
		if f := strings.Fields(lastLine(out)); len(f) >= 5 {
			used, _ := strconv.ParseFloat(f[2], 64)
			total, _ := strconv.ParseFloat(f[1], 64)
			fmt.Fprintf(&b, "Хранилище /overlay: занято %s из %s (%s)\n", fmtKB(used), fmtKB(total), f[4])
		}
	}
	if t := s.temperatures(ctx); t != "" {
		fmt.Fprintf(&b, "Температура: %s\n", t)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// temperatures reads every thermal zone in one command.
func (s Service) temperatures(ctx context.Context) string {
	out, err := s.r.Run(ctx, "ls", "/sys/class/thermal")
	if err != nil {
		return ""
	}
	var files []string
	for _, z := range strings.Fields(out) {
		if strings.HasPrefix(z, "thermal_zone") {
			files = append(files, "/sys/class/thermal/"+z+"/type", "/sys/class/thermal/"+z+"/temp")
		}
	}
	if len(files) == 0 {
		return ""
	}
	return s.readTemps(ctx, files)
}

func (s Service) readTemps(ctx context.Context, files []string) string {
	out, err := s.r.Run(ctx, "grep", append([]string{"-H", ""}, files...)...)
	if err != nil {
		return ""
	}
	vals := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			vals[line[:i]] = strings.TrimSpace(line[i+1:])
		}
	}
	var parts []string
	for i := 0; i+1 < len(files); i += 2 {
		typ, raw := vals[files[i]], vals[files[i+1]]
		milli, err := strconv.ParseFloat(raw, 64)
		if err != nil || typ == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %.0f °C", typ, milli/1000))
	}
	if len(parts) > 6 {
		parts = parts[:6]
	}
	return strings.Join(parts, ", ")
}

func (s Service) Reboot(ctx context.Context) (string, error) {
	if _, err := s.r.Run(ctx, "reboot"); err != nil && !connectionDropped(err) {
		return "", err
	}
	return "Перезагрузка запущена. Роутер будет недоступен около минуты.", nil
}

// connectionDropped: over SSH a reboot ends the session without an exit status.
func connectionDropped(err error) bool {
	m := err.Error()
	return strings.Contains(m, "without exit status") || strings.Contains(m, "EOF") || strings.Contains(m, "closed")
}

// --------------------------------------------------------------- services

func (s Service) initScripts(ctx context.Context) ([]string, error) {
	out, err := s.r.Run(ctx, "ls", "/etc/init.d")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	sort.Strings(names)
	return names, nil
}

func (s Service) Services(ctx context.Context) (string, error) {
	names, err := s.initScripts(ctx)
	if err != nil {
		return "", err
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
	lines := []string{"Службы (● работает, ○ нет данных о работе; авто = запускается при загрузке):"}
	for _, n := range names {
		mark := "○"
		if running[n] {
			mark = "●"
		}
		auto := ""
		if enabled[n] {
			auto = "  авто"
		}
		lines = append(lines, fmt.Sprintf("%s %s%s", mark, n, auto))
	}
	lines = append(lines, "", "Управление: /service <имя> start|stop|restart|reload|enable|disable")
	return strings.Join(lines, "\n"), nil
}

var serviceActions = map[string]bool{"start": true, "stop": true, "restart": true, "reload": true, "enable": true, "disable": true}

func (s Service) ServiceAction(ctx context.Context, name, action string) (string, error) {
	if !ValidService(name) {
		return "", fmt.Errorf("недопустимое имя службы")
	}
	if !serviceActions[action] {
		return "", fmt.Errorf("действие: start, stop, restart, reload, enable или disable")
	}
	if name == selfService && (action == "stop" || action == "restart") {
		return "", fmt.Errorf("бот не может остановить или перезапустить сам себя, сделайте это в LuCI или по SSH")
	}
	names, err := s.initScripts(ctx)
	if err != nil {
		return "", err
	}
	found := false
	for _, n := range names {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("службы %q нет в /etc/init.d, список: /services", name)
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, "/etc/init.d/"+name, action)
	if err != nil {
		return "", err
	}
	res := fmt.Sprintf("%s %s: выполнено", name, action)
	if out != "" {
		res += "\n" + clip(Redact(out), 1500)
	}
	return res, nil
}

// ------------------------------------------------------------------- logs

func (s Service) Syslog(ctx context.Context, n int) (string, error) {
	n = clampLines(n)
	out, err := s.r.Run(ctx, "logread", "-l", strconv.Itoa(n))
	if err != nil {
		return "", err
	}
	if out == "" {
		return "Журнал пуст.", nil
	}
	return clip(Redact(tailLines(out, n)), 12000), nil
}

func (s Service) Dmesg(ctx context.Context, n int) (string, error) {
	n = clampLines(n)
	out, err := s.r.Run(ctx, "dmesg")
	if err != nil {
		return "", err
	}
	return clip(Redact(tailLines(out, n)), 12000), nil
}

func clampLines(n int) int {
	switch {
	case n <= 0:
		return 50
	case n > 500:
		return 500
	}
	return n
}

// ------------------------------------------------------------ ping, trace

func (s Service) Ping(ctx context.Context, host string) (string, error) {
	if !ValidHost(host) {
		return "", fmt.Errorf("недопустимый адрес")
	}
	cctx, cancel := withBudget(ctx, pingTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, "ping", "-c", "4", "-W", "2", strings.Trim(host, "[]"))
	if err != nil {
		return "", fmt.Errorf("%s не отвечает: %v", host, shortErr(err))
	}
	return tailLines(out, 8), nil
}

func (s Service) Traceroute(ctx context.Context, host string) (string, error) {
	if !ValidHost(host) {
		return "", fmt.Errorf("недопустимый адрес")
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, "traceroute", "-m", "15", "-w", "2", "-q", "1", strings.Trim(host, "[]"))
	if err != nil {
		return "", fmt.Errorf("traceroute: %v", shortErr(err))
	}
	return out, nil
}

func shortErr(err error) string {
	m := err.Error()
	if i := strings.LastIndex(m, ": "); i >= 0 && i+2 < len(m) {
		m = m[i+2:]
	}
	if len(m) > 200 {
		m = m[:200]
	}
	return m
}

func isIP(s string) bool { return net.ParseIP(strings.Trim(s, "[]")) != nil }
