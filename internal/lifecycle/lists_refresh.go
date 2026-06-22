package lifecycle

import (
	"fmt"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// RefreshListsWithMonitor reloads nft rules and asks the monitor to pick up list file changes.
func RefreshListsWithMonitor(uciPath string) error {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return fmt.Errorf("load uci: %w", err)
	}
	if err := netlink.ApplyFromUCI(pkg); err != nil {
		return fmt.Errorf("nft setup: %w", err)
	}
	if err := RefreshPerClient(uciPath); err != nil {
		return err
	}
	return RequestEngineSync()
}
