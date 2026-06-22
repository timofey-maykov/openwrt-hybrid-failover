package lifecycle

import (
	"context"
	"os/exec"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
)

type Watchdog struct {
	Interval time.Duration
	Probe    func() error
	Restart  func() error
}

func DefaultWatchdog(uciPath string) *Watchdog {
	_ = uciPath
	return &Watchdog{
		Interval: 15 * time.Second,
		Probe: func() error {
			if engine.Alive() {
				return nil
			}
			return exec.ErrNotFound
		},
		Restart: func() error {
			engine.Default().Stop()
			return nil
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
			_ = netlink.EnsureIPRules()
			_ = dnsmasq.EnsureLocalResolv()
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
