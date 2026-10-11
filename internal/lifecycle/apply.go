package lifecycle

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lists"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/perclient"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subscription"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Options struct {
	UCIPath string
	DryRun  bool
}

type Result struct {
	ConfigHash string
	Changed    bool
}

func Apply(opts Options) (Result, error) {
	if opts.UCIPath == "" {
		opts.UCIPath = paths.UCIConfig
	}

	pkg, err := uci.Load(opts.UCIPath)
	if err != nil {
		return Result{}, fmt.Errorf("load uci: %w", err)
	}
	if len(pkg.SectionNames("section")) == 0 {
		return Result{}, fmt.Errorf("no routing section in UCI")
	}

	awg2Synced := false
	if !opts.DryRun {
		var err error
		awg2Synced, err = SetupAWG2FromUCI(pkg)
		if err != nil {
			return Result{}, err
		}
		if awg2Synced {
			if pkg2, err := uci.Load(opts.UCIPath); err == nil {
				pkg = pkg2
			}
		}
	}

	if !engine.NativeEnabled(pkg) {
		return Result{}, fmt.Errorf("engine_mode=singbox removed; run hybrid-failover migrate")
	}
	if opts.DryRun {
		p, err := engine.CompilePlan(pkg)
		if err != nil {
			return Result{}, fmt.Errorf("compile engine plan: %w", err)
		}
		if err := engine.ValidatePlan(p); err != nil {
			return Result{}, err
		}
		hashStr := plan.Hash(p)
		oldHash, _ := os.ReadFile(enginePlanHashPath)
		changed := string(oldHash) != hashStr+"\n" || awg2Synced
		return Result{ConfigHash: hashStr, Changed: changed}, nil
	}
	res, err := applyNativeEngine(opts.UCIPath)
	if err != nil {
		return Result{}, err
	}
	if awg2Synced {
		res.Changed = true
	}
	return res, nil
}

// ApplyAndReloadIfChanged runs Apply and reloads the native engine when the config hash changed.
func ApplyAndReloadIfChanged(opts Options) (Result, error) {
	res, err := Apply(opts)
	if err != nil {
		return res, err
	}
	// The cron jobs follow the settings, not the plan: a new subscription URL
	// or interval leaves the plan hash alone, so sync them on every apply,
	// not only at start.
	if !opts.DryRun {
		syncCron(opts.UCIPath)
	}
	if !res.Changed {
		return res, nil
	}
	if err := reloadNativeEngine(context.Background(), opts.UCIPath); err != nil {
		return res, err
	}
	return res, nil
}

// RefreshPerClient reloads per-client nft rules from UCI.
func RefreshPerClient(uciPath string) error {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return fmt.Errorf("load uci: %w", err)
	}
	return perclient.RefreshFromUCI(pkg)
}

func syncCron(uciPath string) {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	_ = lists.InstallCron(uciPath)
	if err := subscription.InstallCron(uciPath); err != nil {
		log.Printf("hybrid-failover apply: subscription cron: %v", err)
	}
}
