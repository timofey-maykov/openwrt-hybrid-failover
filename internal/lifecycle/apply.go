package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/perclient"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Options struct {
	UCIPath       string
	ConfigPath    string
	SingboxInit   string
	DryRun        bool
	SkipSingbox   bool
}

type Result struct {
	ConfigHash string
	Changed    bool
}

func Apply(opts Options) (Result, error) {
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
		return Result{}, fmt.Errorf("load uci: %w", err)
	}
	if !pkg.HasOutboundSection() {
		return Result{}, fmt.Errorf("no outbound section in UCI")
	}

	builder := singbox.NewBuilder(pkg)
	cfg, err := builder.Build()
	if err != nil {
		return Result{}, fmt.Errorf("build sing-box config: %w", err)
	}
	data, err := cfg.JSON()
	if err != nil {
		return Result{}, err
	}
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	oldHash, _ := os.ReadFile(opts.ConfigPath + ".sha256")
	changed := string(oldHash) != hashStr+"\n"

	if opts.DryRun {
		return Result{ConfigHash: hashStr, Changed: changed}, nil
	}

	if !opts.SkipSingbox {
		if err := singbox.CheckMinimumVersion(); err != nil {
			return Result{}, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.ConfigPath), 0o755); err != nil {
		return Result{}, err
	}
	tmp := opts.ConfigPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return Result{}, err
	}
	out, err := exec.Command("sing-box", "check", "-c", tmp).CombinedOutput()
	if err != nil {
		_ = os.Remove(tmp)
		return Result{}, fmt.Errorf("sing-box check failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, opts.ConfigPath); err != nil {
		return Result{}, err
	}
	_ = os.WriteFile(opts.ConfigPath+".sha256", []byte(hashStr+"\n"), 0o644)
	return Result{ConfigHash: hashStr, Changed: changed}, nil
}

func ReloadSingbox(init string) error {
	if init == "" {
		init = paths.SingboxInit
	}
	out, err := exec.Command(init, "reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box reload: %w: %s", err, string(out))
	}
	return nil
}

// ApplyAndReloadIfChanged runs Apply and reloads sing-box when the config hash changed.
func ApplyAndReloadIfChanged(opts Options) (Result, error) {
	res, err := Apply(opts)
	if err != nil {
		return res, err
	}
	if res.Changed {
		if err := ReloadSingbox(opts.SingboxInit); err != nil {
			return res, err
		}
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

func StopSingbox(init string) error {
	if init == "" {
		init = paths.SingboxInit
	}
	out, err := exec.Command(init, "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box stop: %w: %s", err, string(out))
	}
	return nil
}
