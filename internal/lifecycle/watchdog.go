package lifecycle

import (
	"context"
	"os/exec"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
)

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
			_ = netlink.EnsureIPRules()
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
