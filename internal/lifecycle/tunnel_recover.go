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

// RecoverStaleTunnels recreates mismatched AWG2 peers and bounces primary WG ifaces.
// Returns true when at least one interface was changed.
func RecoverStaleTunnels(uciPath string) bool {
	return recoverStaleTunnels(uciPath, nil)
}

type tunnelRecoverHooks struct {
	skipBounce func(iface string) bool
	onBounced  func(iface string)
	onHealthy  func(iface string)
}

func recoverStaleTunnels(uciPath string, hooks *tunnelRecoverHooks) bool {
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
		log.Printf("hybrid-failover watchdog: recreated mismatched awg2 interface(s)")
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
		if !shouldBounceStaleTunnel(iface) {
			continue
		}
		if hooks != nil && hooks.skipBounce != nil && hooks.skipBounce(iface) {
			continue
		}
		stale, bounced := recoverIfStaleWG(iface)
		if !stale {
			if hooks != nil && hooks.onHealthy != nil {
				hooks.onHealthy(iface)
			}
			continue
		}
		if bounced {
			changed = true
			if hooks != nil && hooks.onBounced != nil {
				hooks.onBounced(iface)
			}
		}
	}
	return changed
}

// shouldBounceStaleTunnel is false for netifd VPN (awg0) and for pawg* (SetupAWG2FromUCI).
// Flapping a netifd AmneziaWG peer with allowed-ips 0.0.0.0/0 can drop LAN/WAN routing.
func shouldBounceStaleTunnel(iface string) bool {
	_ = iface
	return false
}

// recoverIfStaleWG returns (wasStale, bouncedOK).
func recoverIfStaleWG(ifname string) (stale bool, bounced bool) {
	if ifname == "" {
		return false, false
	}
	if out, err := exec.Command("ip", "link", "show", "dev", ifname).CombinedOutput(); err != nil {
		return false, false
	} else if !strings.Contains(string(out), "UP") {
		return false, false
	}
	fresh, detail := probe.WgHandshakeFresh(ifname, probe.DefaultWGHandshakeMaxAge)
	if fresh || detail == "" {
		return false, false
	}
	log.Printf("hybrid-failover watchdog: %s unhealthy (%s), bouncing", ifname, detail)
	if err := bounceAWGInterface(ifname); err != nil {
		log.Printf("hybrid-failover watchdog: bounce %s: %v", ifname, err)
		return true, false
	}
	return true, true
}

func (w *Watchdog) recoverStaleTunnels() {
	if time.Since(w.lastTunnelRecover) < 2*time.Minute {
		return
	}
	// Always advance the timer so SetupAWG2 cannot run every watchdog tick when
	// nothing changed (previous bug: uci commit network every 10s → lost default route).
	w.lastTunnelRecover = time.Now()
	_ = recoverStaleTunnels(w.UCIPath, &tunnelRecoverHooks{
		skipBounce: w.skipWGBounce,
		onBounced:  w.markWGBounced,
		onHealthy:  w.markWGHealthy,
	})
}

func (w *Watchdog) skipWGBounce(iface string) bool {
	until, ok := w.wgBounceCooldown[iface]
	return ok && time.Now().Before(until)
}

func (w *Watchdog) markWGHealthy(iface string) {
	delete(w.wgBounceFail, iface)
	delete(w.wgBounceCooldown, iface)
}

func (w *Watchdog) markWGBounced(iface string) {
	if w.wgBounceFail == nil {
		w.wgBounceFail = make(map[string]int)
	}
	if w.wgBounceCooldown == nil {
		w.wgBounceCooldown = make(map[string]time.Time)
	}
	n := w.wgBounceFail[iface] + 1
	w.wgBounceFail[iface] = n
	cooldown := time.Duration(n) * 5 * time.Minute
	if cooldown > 30*time.Minute {
		cooldown = 30 * time.Minute
	}
	w.wgBounceCooldown[iface] = time.Now().Add(cooldown)
	log.Printf("hybrid-failover watchdog: %s bounce cooldown %s", iface, cooldown)
}
