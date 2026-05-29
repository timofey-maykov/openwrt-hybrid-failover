package lifecycle

import (
	"fmt"
	"os/exec"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lists"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type StartOptions struct {
	UCIPath     string
	ConfigPath  string
	SingboxInit string
}

type StartResult struct {
	ConfigHash string
}

// StartPipeline runs the full Hybrid Failover startup sequence.
func StartPipeline(opts StartOptions) (StartResult, error) {
	if opts.UCIPath == "" {
		opts.UCIPath = paths.UCIConfig
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = paths.SingboxConfig
	}
	if opts.SingboxInit == "" {
		opts.SingboxInit = paths.SingboxInit
	}

	pkg, err := uci.Load(opts.UCIPath)
	if err != nil {
		return StartResult{}, fmt.Errorf("load uci: %w", err)
	}

	if err := SetupAWG2FromUCI(pkg); err != nil {
		return StartResult{}, err
	}
	if pkg2, err := uci.Load(opts.UCIPath); err == nil {
		pkg = pkg2
	}

	applyOpts := Options{
		UCIPath:     opts.UCIPath,
		ConfigPath:  opts.ConfigPath,
		SingboxInit: opts.SingboxInit,
	}
	res, err := Apply(applyOpts)
	if err != nil {
		return StartResult{}, err
	}

	if err := netlink.ApplyFromUCI(pkg); err != nil {
		return StartResult{}, fmt.Errorf("nft setup: %w", err)
	}

	if err := ReloadSingbox(opts.SingboxInit); err != nil {
		if err2 := startSingbox(opts.SingboxInit); err2 != nil {
			return StartResult{}, fmt.Errorf("sing-box start: %w (reload: %v)", err2, err)
		}
	}

	settings := pkg.Section("settings")
	if settings == nil || !settings.GetBool("dont_touch_dhcp", false) {
		_ = dnsmasq.Configure()
	}

	updater := lists.NewUpdater(false)
	if err := updater.UpdateOnce(); err == nil {
		if res2, err := ApplyAndReloadIfChanged(applyOpts); err != nil {
			return StartResult{}, err
		} else if res2.Changed {
			res = res2
		}
	}

	_ = lists.InstallCron(opts.UCIPath)

	return StartResult{ConfigHash: res.ConfigHash}, nil
}

func startSingbox(init string) error {
	out, err := exec.Command(init, "start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", init, err, string(out))
	}
	return nil
}
