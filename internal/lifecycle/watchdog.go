package lifecycle

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lists"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Watchdog struct {
	UCIPath  string
	Interval time.Duration
	Probe    func() error
	Restart  func() error

	lastListRecover time.Time
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
			// Nudge the engine loop to restart even when already stopped.
			_ = RequestEngineSync()
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

// recoverEmptyLists re-downloads community rulesets and forces monitor reload when stubs are empty.
func (w *Watchdog) recoverEmptyLists() {
	if w.UCIPath == "" {
		return
	}
	if time.Since(w.lastListRecover) < 2*time.Minute {
		return
	}
	updater := lists.NewFromUCI(w.UCIPath)
	if updater.HasValidCache() {
		return
	}
	w.lastListRecover = time.Now()
	go func() {
		log.Printf("hybrid-failover watchdog: community lists empty, updating")
		if _, err := updater.UpdateOnce(); err != nil {
			log.Printf("hybrid-failover watchdog: list update: %v", err)
			if !updater.HasValidCache() {
				return
			}
		}
		if err := RefreshListsWithMonitor(w.UCIPath); err != nil {
			log.Printf("hybrid-failover watchdog: refresh after list update: %v", err)
			_ = RequestEngineSync()
		}
	}()
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
			w.recoverEmptyLists()
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
