package lifecycle

import (
	"context"
	"os/exec"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type FailoverWatcher struct {
	UCIPath  string
	Interval time.Duration
	ClashURL string
	Webhook  string
	last     map[string]string
}

func DefaultFailoverWatcher(uciPath string) *FailoverWatcher {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	w := &FailoverWatcher{
		UCIPath:  uciPath,
		Interval: 15 * time.Second,
		last:     make(map[string]string),
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		w.ClashURL = "http://" + clash.DetectListenAddress("")
		return w
	}
	settings := pkg.Section("settings")
	webhook := ""
	if settings != nil {
		w.ClashURL = clash.ResolveBaseURL(settings)
		webhook = settings.Get("webhook_url", "")
		if webhook == "" {
			webhook = settings.Get("failover_webhook_url", "")
		}
	} else {
		w.ClashURL = "http://" + clash.DetectListenAddress("")
	}
	w.Webhook = webhook
	return w
}

func (w *FailoverWatcher) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.Interval <= 0 {
		w.Interval = 15 * time.Second
	}
	if w.last == nil {
		w.last = make(map[string]string)
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollOnce()
		}
	}
}

func (w *FailoverWatcher) pollOnce() {
	pkg, err := uci.Load(w.UCIPath)
	if err != nil {
		return
	}
	sections := failoverWatchSections(pkg)
	if len(sections) == 0 {
		return
	}
	cli := clash.New(w.ClashURL, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, section := range sections {
		selectorTag := singbox.OutboundTag(section)
		active, err := cli.ActiveOutbound(ctx, selectorTag)
		if err != nil || active == "" {
			continue
		}
		prev, ok := w.last[section]
		if ok && prev != active {
			reason := "urltest failover"
			_ = notify.RecordFailover(section, prev, active, reason)
			ev := notify.Event{
				Time:    time.Now().UTC(),
				Section: section,
				From:    prev,
				To:      active,
				Reason:  reason,
			}
			_ = notify.SendWebhook(w.Webhook, ev)
		}
		w.last[section] = active
	}
}

func failoverWatchSections(pkg *uci.Package) []string {
	if pkg == nil {
		return nil
	}
	var names []string
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		switch sec.Get("connection_type", "") {
		case "vpn":
			if sec.GetBool("failover_vpn_enabled", false) && len(sec.GetList("failover_proxy_links")) > 0 {
				names = append(names, name)
			}
		case "proxy":
			if sec.Get("proxy_config_type", "") == "urltest" {
				names = append(names, name)
			}
		}
	}
	return names
}

type Watchdog struct {
	Interval time.Duration
	Probe    func() error
	Restart  func() error
}

func DefaultWatchdog() *Watchdog {
	return &Watchdog{
		Interval: 30 * time.Second,
		Probe: func() error {
			out, err := exec.Command("pidof", "sing-box").CombinedOutput()
			if err != nil || len(out) == 0 {
				return err
			}
			return nil
		},
		Restart: func() error {
			return exec.Command("/etc/init.d/sing-box", "restart").Run()
		},
	}
}

func (w *Watchdog) Run(ctx context.Context) {
	if w.Interval <= 0 {
		w.Interval = 30 * time.Second
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	backoff := w.Interval
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Probe(); err != nil {
				_ = w.Restart()
				time.Sleep(backoff)
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			} else {
				backoff = w.Interval
			}
		}
	}
}
