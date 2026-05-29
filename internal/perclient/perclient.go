package perclient

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// RefreshFromUCI rebuilds nft rules including per-client include/exclude lists.
func RefreshFromUCI(pkg *uci.Package) error {
	return netlink.ApplyFromUCI(pkg)
}

// ApplyFromUCI is an alias for RefreshFromUCI (idempotent).
func ApplyFromUCI(pkg *uci.Package) error {
	return RefreshFromUCI(pkg)
}
