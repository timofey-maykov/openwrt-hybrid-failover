package podkop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/podkop-hybrid-failover/bot/internal/routerexec"
	"github.com/tmaykov/podkop-hybrid-failover/bot/internal/validation"
)

type Service struct {
	runner    routerexec.Runner
	clashAPI  string
	initScript string
	httpCli   *http.Client
}

type ChannelHealth struct {
	Name      string
	Available bool
	DelayMs   int
	Detail    string
}

func NewService(runner routerexec.Runner, clashAPI, initScript string, timeout time.Duration) Service {
	return Service{
		runner:    runner,
		clashAPI:  strings.TrimRight(clashAPI, "/"),
		initScript: initScript,
		httpCli: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s Service) Status(ctx context.Context) (string, error) {
	var lines []string
	out, err := s.runner.Run(ctx, s.initScript, "status")
	if err != nil {
		// OpenWrt init.d often returns exit code 5 for "not running".
		if strings.Contains(err.Error(), "exit status 5") {
			lines = append(lines, "podkop init.d: not running (oneshot mode)")
		} else {
			return "", err
		}
	} else {
		lines = append(lines, "podkop init.d: "+out)
	}

	if out != "" && !strings.Contains(out, "not running") {
		lines = append(lines, out)
	}

	singboxState := "stopped"
	if s.isSingBoxRunning(ctx) {
		singboxState = "running"
	}
	lines = append(lines, "sing-box: "+singboxState)

	proxy, perr := s.CurrentProxy(ctx)
	if perr == nil && proxy != "" {
		lines = append(lines, "clash api: running")
		lines = append(lines, "active_outbound: "+proxy)
	} else {
		lines = append(lines, "clash api: unavailable")
	}

	if singboxState != "running" && perr != nil {
		lines = append(lines, "podkop state: inactive")
	} else {
		lines = append(lines, "podkop state: active")
	}

	if health, herr := s.ChannelHealth(ctx); herr == nil {
		var down []string
		for _, ch := range health {
			if !ch.Available {
				down = append(down, ch.Name)
			}
		}
		if len(down) > 0 {
			lines = append(lines, "warning: недоступны каналы -> "+strings.Join(down, ", "))
		} else if len(health) > 0 {
			lines = append(lines, "каналы: все доступны")
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (s Service) Restart(ctx context.Context) error {
	_, err := s.runner.Run(ctx, s.initScript, "restart")
	return err
}

func (s Service) Apply(ctx context.Context) error {
	_, err := s.runner.Run(ctx, "/sbin/uci", "commit", "podkop")
	if err != nil {
		return err
	}
	return s.Restart(ctx)
}

func (s Service) Rollback(ctx context.Context) error {
	_, err := s.runner.Run(ctx, "/sbin/uci", "revert", "podkop")
	if err != nil {
		return err
	}
	return s.Restart(ctx)
}

func (s Service) ListFailover(ctx context.Context) (string, error) {
	return s.runner.Run(ctx, "/sbin/uci", "get", "podkop.glob.failover_proxy_links")
}

func (s Service) ListRouterParams(ctx context.Context) (string, error) {
	return s.runner.Run(ctx, "/sbin/uci", "show", "podkop")
}

func (s Service) ListRouterSections(ctx context.Context) (string, error) {
	// Use shell tools on-device to avoid parsing multiline list values in Go.
	cmd := "uci show podkop | cut -d= -f1 | cut -d. -f1-2 | sort -u"
	return s.runner.Run(ctx, "/bin/sh", "-lc", cmd)
}

func (s Service) ShowRouterSection(ctx context.Context, section string) (string, error) {
	if err := validation.ValidateUCISectionKey(section); err != nil {
		return "", err
	}
	return s.runner.Run(ctx, "/sbin/uci", "show", section)
}

func (s Service) GetRouterParam(ctx context.Context, key string) (string, error) {
	if err := validation.ValidateUCIKey(key); err != nil {
		return "", err
	}
	return s.runner.Run(ctx, "/sbin/uci", "get", key)
}

func (s Service) SetRouterParam(ctx context.Context, key, value string) error {
	if err := validation.ValidateUCIKey(key); err != nil {
		return err
	}
	value = validation.NormalizeValue(value)
	if value == "" {
		return fmt.Errorf("empty value is not allowed, use delete command")
	}
	quoted := shellQuote(value)
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci set %s=%s", key, quoted))
	return err
}

func (s Service) DelRouterParam(ctx context.Context, key string) error {
	if err := validation.ValidateUCIKey(key); err != nil {
		return err
	}
	_, err := s.runner.Run(ctx, "/sbin/uci", "delete", key)
	return err
}

func (s Service) AddListRouterParam(ctx context.Context, key, value string) error {
	if err := validation.ValidateUCIKey(key); err != nil {
		return err
	}
	value = validation.NormalizeValue(value)
	if value == "" {
		return fmt.Errorf("empty value is not allowed")
	}
	quoted := shellQuote(value)
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci add_list %s=%s", key, quoted))
	return err
}

func (s Service) DelListRouterParam(ctx context.Context, key, value string) error {
	if err := validation.ValidateUCIKey(key); err != nil {
		return err
	}
	value = validation.NormalizeValue(value)
	if value == "" {
		return fmt.Errorf("empty value is not allowed")
	}
	quoted := shellQuote(value)
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci del_list %s=%s", key, quoted))
	return err
}

func (s Service) PendingChanges(ctx context.Context) (string, error) {
	out, err := s.runner.Run(ctx, "/sbin/uci", "changes", "podkop")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "изменений нет", nil
	}
	return out, nil
}

func (s Service) AddFailover(ctx context.Context, uri string) error {
	if err := validation.ValidateProxyURI(uri); err != nil {
		return err
	}
	escaped := strings.ReplaceAll(uri, "'", "'\\''")
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci add_list podkop.glob.failover_proxy_links='%s'", escaped))
	return err
}

func (s Service) RemoveFailover(ctx context.Context, uri string) error {
	escaped := strings.ReplaceAll(uri, "'", "'\\''")
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci del_list podkop.glob.failover_proxy_links='%s'", escaped))
	return err
}

func (s Service) SwitchOutbound(ctx context.Context, outbound string) error {
	endpoint := s.clashAPI + "/proxies/glob-out"
	values := url.Values{}
	values.Set("name", outbound)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("switch outbound failed: status %s", resp.Status)
	}
	return nil
}

func (s Service) CurrentProxy(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.clashAPI+"/proxies/glob-out", nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("clash api status %s", resp.Status)
	}

	var payload struct {
		Now string `json:"now"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Now, nil
}

func (s Service) Logs(ctx context.Context, lines int) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	if lines > 500 {
		lines = 500
	}
	return s.runner.Run(ctx, "/sbin/logread", "-e", "podkop", "-l", fmt.Sprintf("%d", lines))
}

func (s Service) ChannelHealth(ctx context.Context) ([]ChannelHealth, error) {
	names, err := s.globOutboundNames(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "connect: connection refused") {
			return nil, fmt.Errorf("clash api недоступен (%s). Вероятно podkop/sing-box не запущен", s.clashAPI)
		}
		return nil, err
	}
	result := make([]ChannelHealth, 0, len(names))
	for _, name := range names {
		ch := ChannelHealth{Name: name}
		delay, derr := s.delayProbe(ctx, name)
		if derr != nil {
			ch.Available = false
			ch.Detail = derr.Error()
		} else {
			ch.Available = true
			ch.DelayMs = delay
			ch.Detail = fmt.Sprintf("%dms", delay)
		}
		result = append(result, ch)
	}
	return result, nil
}

func shellQuote(in string) string {
	return "'" + strings.ReplaceAll(in, "'", "'\\''") + "'"
}

func (s Service) isSingBoxRunning(ctx context.Context) bool {
	_, err := s.runner.Run(ctx, "/bin/sh", "-lc", "pgrep -f 'sing-box' >/dev/null 2>&1 && echo ok")
	return err == nil
}

func (s Service) globOutboundNames(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.clashAPI+"/proxies", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("clash api status %s", resp.Status)
	}
	var payload struct {
		Proxies map[string]json.RawMessage `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var names []string
	for name := range payload.Proxies {
		if strings.HasPrefix(name, "glob-") && strings.HasSuffix(name, "-out") && name != "glob-out" && name != "glob-urltest-out" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (s Service) delayProbe(ctx context.Context, outbound string) (int, error) {
	probeURL := s.clashAPI + "/proxies/" + url.PathEscape(outbound) + "/delay?timeout=5000&url=https://www.gstatic.com/generate_204"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("probe status %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if v, ok := payload["delay"]; ok {
		switch d := v.(type) {
		case float64:
			return int(d), nil
		case string:
			n, _ := strconv.Atoi(d)
			if n > 0 {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("no delay in response")
}
