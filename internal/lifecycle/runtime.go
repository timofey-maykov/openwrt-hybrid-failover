package lifecycle

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lanipv6"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

var (
	bgCancel              context.CancelFunc
	dnsmasqConfigureCancel context.CancelFunc
)

// StartBackground runs native engine (when enabled), failover controller, and watchdog until CancelBackground.
func StartBackground(uciPath string) {
	CancelBackground()
	ctx, cancel := context.WithCancel(context.Background())
	bgCancel = cancel
	initEnginePlanHashFromDisk()
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	if pkg, err := uci.Load(uciPath); err == nil && engine.NativeEnabled(pkg) {
		go runNativeEngineSupervisor(ctx, uciPath)
		go runEnginePlanSyncLoop(ctx, uciPath)
		go runResolvGuardLoop(ctx, uciPath)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "hybrid-failover engine: uci load: %v\n", err)
	}
	go DefaultFailoverController(uciPath).Run(ctx)
	go DefaultWatchdog(uciPath).Run(ctx)
	go runMemoryScavengeLoop(ctx)
	if shouldManageDNSMasq(uciPath) {
		_ = dnsmasq.EnsureLocalResolv()
	}
}

// runNativeEngineSupervisor keeps the engine loop alive across panics / unexpected exits.
// Native-disabled (p==nil) is handled inside the loop with a slow poll, not a restart storm.
func runNativeEngineSupervisor(ctx context.Context, uciPath string) {
	for {
		if ctx.Err() != nil {
			return
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "hybrid-failover engine: panic: %v\n%s\n", r, debug.Stack())
				}
			}()
			runNativeEngineLoop(ctx, uciPath)
		}()
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "hybrid-failover engine: loop exited, restarting in 2s\n")
		if !sleepOrDone(ctx, 2*time.Second) {
			return
		}
	}
}

func runNativeEngineLoop(ctx context.Context, uciPath string) {
	if err := netlink.EnsureDNSBindAddr(); err != nil {
		log.Printf("hybrid-failover engine: dns bind addr: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			engine.Default().Stop()
			return
		default:
		}

		p, hash, err := compileEnginePlan(uciPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hybrid-failover engine: compile plan: %v\n", err)
			if !sleepOrDone(ctx, 5*time.Second) {
				return
			}
			continue
		}
		if p == nil {
			// Native mode off in UCI: keep the supervisor idle instead of exiting.
			if !sleepOrDone(ctx, 30*time.Second) {
				return
			}
			continue
		}

		if shouldManageDNSMasq(uciPath) {
			// After Configure, dnsmasq uses notinterface=lo and must stay up for LAN DNS.
			// Only stop it when it may still own 127.0.0.42:53 (pre-Configure / after Restore).
			if !dnsmasq.UsesEngineUpstream() {
				if err := dnsmasq.StopService(); err != nil {
					fmt.Fprintf(os.Stderr, "hybrid-failover engine: dnsmasq stop: %v\n", err)
				}
			}
		}

		eng := engine.Default()
		eng.Stop()
		if err := eng.ApplyPlan(p); err != nil {
			fmt.Fprintf(os.Stderr, "hybrid-failover engine: apply plan: %v\n", err)
			if !sleepOrDone(ctx, 5*time.Second) {
				return
			}
			continue
		}
		NoteEnginePlanHash(hash)
		_ = os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644)

		if dnsmasqConfigureCancel != nil {
			dnsmasqConfigureCancel()
		}
		cfgCtx, cfgCancel := context.WithCancel(ctx)
		dnsmasqConfigureCancel = cfgCancel
		go configureDNSMasqWhenReady(cfgCtx, uciPath)

		if err := eng.Run(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "hybrid-failover engine: run: %v\n", err)
		}
		_ = engine.WriteRuntimeSnapshot(eng.Snapshot())

		if ctx.Err() != nil {
			return
		}
		// Brief FakeIP gap during restart is OK; do not Restore dnsmasq here
		// (watchdog restores only after sustained DNS outage).
		if !sleepOrDone(ctx, 2*time.Second) {
			return
		}
	}
}

func shouldManageDNSMasq(uciPath string) bool {
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return true
	}
	settings := pkg.Section("settings")
	return settings == nil || !settings.GetBool("dont_touch_dhcp", false)
}

func configureDNSMasqWhenReady(ctx context.Context, uciPath string) {
	if !shouldManageDNSMasq(uciPath) {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !engine.DNSReady() {
				continue
			}
			if err := lanipv6.ApplyFromUCI(uciPath); err != nil {
				fmt.Fprintf(os.Stderr, "hybrid-failover engine: lan ipv6: %v\n", err)
			}
			// Already forwarding to FakeIP: keep dnsmasq process, do not restart.
			if dnsmasq.UsesEngineUpstream() {
				_ = dnsmasq.EnsureRunning()
				_ = dnsmasq.EnsureLocalResolvIfNeeded()
				return
			}
			if err := dnsmasq.Configure(); err != nil {
				fmt.Fprintf(os.Stderr, "hybrid-failover engine: dnsmasq configure: %v\n", err)
				return
			}
			go func() {
				for i := 0; i < 10; i++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					_ = dnsmasq.EnsureLocalResolvIfNeeded()
					time.Sleep(500 * time.Millisecond)
				}
			}()
			return
		}
	}
}

func runResolvGuardLoop(ctx context.Context, uciPath string) {
	if !shouldManageDNSMasq(uciPath) {
		return
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		_ = dnsmasq.EnsureLocalResolvIfNeeded()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runEnginePlanSyncLoop(ctx context.Context, uciPath string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		SyncNativeEnginePlan(ctx, uciPath)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func runMemoryScavengeLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			debug.FreeOSMemory()
		}
	}
}

// CancelBackground stops background controller and watchdog goroutines.
func CancelBackground() {
	if bgCancel != nil {
		bgCancel()
		bgCancel = nil
	}
	stopNativeEngine()
}

func nativeModeFromUCI(uciPath string) bool {
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return plan.LoadEngineMode() == plan.ModeNative
	}
	return engine.NativeEnabled(pkg)
}
