package lifecycle

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Watchdog struct {
	UCIPath  string
	Interval time.Duration
	Probe    func() error
	Restart  func() error
}

func DefaultWatchdog(uciPath string) *Watchdog {
	return &Watchdog{
		UCIPath:  uciPath,
		Interval: 15 * time.Second,
		Probe: func() error {
			if err := netlink.Check(); err != nil {
				return err
			}
			if engine.Alive() {
				return nil
			}
			return os.ErrNotExist
		},
		Restart: func() error {
			engine.Default().Stop()
			return nil
		},
	}
}

func (w *Watchdog) restoreNFT() {
	if err := netlink.Check(); err == nil {
		return
	}
	uciPath := w.UCIPath
	if uciPath == "" {
		return
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		log.Printf("hybrid-failover watchdog: load uci: %v", err)
		return
	}
	if err := netlink.ApplyFromUCI(pkg); err != nil {
		log.Printf("hybrid-failover watchdog: nft restore: %v", err)
	}
}

func (w *Watchdog) Run(ctx context.Context) {
	if w.Interval <= 0 {
		w.Interval = 30 * time.Second
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	backoff := w.Interval
	failStreak := 0
	const failThreshold = 3
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = netlink.EnsureIPRules()
			w.restoreNFT()
			_ = dnsmasq.EnsureLocalResolv()
			if err := w.Probe(); err != nil {
				failStreak++
				if failStreak < failThreshold {
					continue
				}
				_ = w.Restart()
				failStreak = 0
				time.Sleep(backoff)
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			} else {
				failStreak = 0
				backoff = w.Interval
			}
		}
	}
}
