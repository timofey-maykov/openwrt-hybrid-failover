package lifecycle

import (
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/probe"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// RecoverStaleTunnels recreates stale AWG2 peers and bounces primary WG ifaces.
// Returns true when at least one interface was changed.
func RecoverStaleTunnels(uciPath string) bool {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return false
	}

	changed := false
	if synced, err := SetupAWG2FromUCI(pkg); err != nil {
		log.Printf("hybrid-failover watchdog: awg2 setup: %v", err)
	} else if synced {
		changed = true
		log.Printf("hybrid-failover watchdog: recreated stale awg2 interface(s)")
	}

	seen := make(map[string]struct{})
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		iface := strings.TrimSpace(sec.Get("interface", ""))
		if iface == "" {
			continue
		}
		if _, ok := seen[iface]; ok {
			continue
		}
		seen[iface] = struct{}{}
		// Skip ifaces already handled by SetupAWG2FromUCI (pawg*).
		if strings.HasPrefix(iface, "pawg") {
			continue
		}
		if recoverIfStaleWG(iface) {
			changed = true
		}
	}
	return changed
}

func recoverIfStaleWG(ifname string) bool {
	if ifname == "" {
		return false
	}
	if out, err := exec.Command("ip", "link", "show", "dev", ifname).CombinedOutput(); err != nil {
		return false
	} else if !strings.Contains(string(out), "UP") {
		return false
	}
	fresh, detail := probe.WgHandshakeFresh(ifname, probe.DefaultWGHandshakeMaxAge)
	if fresh || detail == "" {
		return false
	}
	log.Printf("hybrid-failover watchdog: %s unhealthy (%s), bouncing", ifname, detail)
	if err := bounceAWGInterface(ifname); err != nil {
		log.Printf("hybrid-failover watchdog: bounce %s: %v", ifname, err)
		return false
	}
	return true
}

func (w *Watchdog) recoverStaleTunnels() {
	if time.Since(w.lastTunnelRecover) < 2*time.Minute {
		return
	}
	if RecoverStaleTunnels(w.UCIPath) {
		w.lastTunnelRecover = time.Now()
	}
}
