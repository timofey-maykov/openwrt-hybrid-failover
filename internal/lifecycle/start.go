package lifecycle

import (
	"fmt"
	"log"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/lanipv6"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lists"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type StartOptions struct {
	UCIPath string
}

type StartResult struct {
	ConfigHash string
}

// StartPipeline runs the full Hybrid Failover startup sequence.
func StartPipeline(opts StartOptions) (StartResult, error) {
	if opts.UCIPath == "" {
		opts.UCIPath = paths.UCIConfig
	}

	pkg, err := uci.Load(opts.UCIPath)
	if err != nil {
		return StartResult{}, fmt.Errorf("load uci: %w", err)
	}

	applyOpts := Options{
		UCIPath: opts.UCIPath,
	}
	res, err := Apply(applyOpts)
	if err != nil {
		return StartResult{}, err
	}

	if err := netlink.ApplyFromUCI(pkg); err != nil {
		return StartResult{}, fmt.Errorf("nft setup: %w", err)
	}

	if err := netlink.EnsureDNSBindAddr(); err != nil {
		return StartResult{}, fmt.Errorf("dns bind addr: %w", err)
	}

	if err := lanipv6.ApplyFromUCI(opts.UCIPath); err != nil {
		return StartResult{}, fmt.Errorf("lan ipv6: %w", err)
	}

	_ = lists.InstallCron(opts.UCIPath)

	updater := lists.NewFromUCI(opts.UCIPath)
	go func() {
		if _, listErr := updater.UpdateOnce(); listErr != nil {
			log.Printf("hybrid-failover start: list update: %v", listErr)
			return
		}
		if _, err := ApplyAndReloadIfChanged(applyOpts); err != nil {
			log.Printf("hybrid-failover start: apply after list update: %v", err)
		}
	}()

	return StartResult{ConfigHash: res.ConfigHash}, nil
}
