package lifecycle

import (
	"fmt"

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

	updater := lists.NewFromUCI(opts.UCIPath)
	_, listErr := updater.UpdateOnce()
	if listErr != nil && !updater.HasValidCache() {
		return StartResult{}, fmt.Errorf("list update: %w", listErr)
	}

	if res2, err := ApplyAndReloadIfChanged(applyOpts); err != nil {
		return StartResult{}, err
	} else if res2.Changed {
		res = res2
	}

	_ = lists.InstallCron(opts.UCIPath)

	return StartResult{ConfigHash: res.ConfigHash}, nil
}
