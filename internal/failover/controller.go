package failover

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/probe"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/policy"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const (
	modePrimary = "primary"
	modeBackup  = "backup"
)

// SectionConfig is one managed failover routing section.
type SectionConfig struct {
	Section        string
	Policy         policy.VPNFailover
	PrimaryTag       string
	PrimaryIface     string
	URLTestTag       string
	SelectorTag      string
	BackupTags       []string
	TestURL          string
	ProbeInterval    time.Duration
	FailThreshold    int
	RecoverThreshold int
	Sec              *uci.Section
}

// SectionRuntime is persisted controller state for status/history.
type SectionRuntime struct {
	Section       string    `json:"section"`
	Policy        string    `json:"policy"`
	Mode          string    `json:"mode"`
	Active        string    `json:"active"`
	PrimaryOK     bool      `json:"primary_ok"`
	PrimaryDelay  int       `json:"primary_delay_ms,omitempty"`
	FailStreak    int       `json:"fail_streak"`
	RecoverStreak int       `json:"recover_streak"`
	LastProbeAt   time.Time `json:"last_probe_at,omitempty"`
	LastSwitchAt  time.Time `json:"last_switch_at,omitempty"`
	ActiveSince   time.Time `json:"active_since,omitempty"`
	LastSwitch    time.Time `json:"last_switch,omitempty"` // deprecated alias
	LastError     string    `json:"last_error,omitempty"`
}

// Controller enforces outage-only / prefer-primary policies via Clash API.
type Controller struct {
	UCIPath  string
	ClashURL string
	Webhook  string
	Interval time.Duration

	sections []SectionConfig
	states   map[string]*sectionState
}

type sectionState struct {
	mode          string
	failStreak    int
	recoverStreak int
	lastActive    string
	activeSince   time.Time
	lastSwitchAt  time.Time
}

func DefaultController(uciPath string) *Controller {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	c := &Controller{
		UCIPath:  uciPath,
		Interval: 30 * time.Second,
		states:   make(map[string]*sectionState),
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		c.ClashURL = "http://" + clash.DetectListenAddress("")
		return c
	}
	if settings := pkg.Section("settings"); settings != nil {
		c.ClashURL = clash.ResolveBaseURL(settings)
		c.Webhook = settings.Get("webhook_url", "")
		if c.Webhook == "" {
			c.Webhook = settings.Get("failover_webhook_url", "")
		}
		c.Interval = parseControllerInterval(settings.Get("failover_probe_interval", ""))
	} else {
		c.ClashURL = "http://" + clash.DetectListenAddress("")
	}
	return c
}

func (c *Controller) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.states == nil {
		c.states = make(map[string]*sectionState)
	}
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()
	c.pollOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollOnce()
		}
	}
}

func (c *Controller) pollOnce() {
	pkg, err := uci.Load(c.UCIPath)
	if err != nil {
		return
	}
	c.sections = loadManagedSections(pkg)
	if len(c.sections) == 0 {
		return
	}

	cli := clash.New(c.ClashURL, 12*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	runtimes := make([]SectionRuntime, 0, len(c.sections))
	for _, sec := range c.sections {
		rt := c.pollSection(ctx, cli, sec)
		runtimes = append(runtimes, rt)
	}
	_ = writeRuntimeState(runtimes)
}

func (c *Controller) pollSection(ctx context.Context, cli *clash.Client, sec SectionConfig) SectionRuntime {
	st := c.stateFor(sec.Section)
	rt := SectionRuntime{
		Section: sec.Section,
		Policy:  string(sec.Policy),
		Mode:    st.mode,
	}

	active, err := cli.ActiveOutbound(ctx, sec.SelectorTag)
	if err != nil {
		rt.LastError = err.Error()
		return rt
	}
	rt.Active = active
	st.lastActive = active

	if sec.Policy == policy.Fastest || sec.PrimaryTag == "" {
		c.recordPassive(sec, st, active)
		rt.Mode = "urltest"
		return rt
	}

	now := time.Now().UTC()
	primaryDelay, primaryOK := probeOutbound(ctx, cli, sec, sec.PrimaryTag, sec.TestURL)
	rt.PrimaryOK = primaryOK
	rt.PrimaryDelay = primaryDelay
	rt.FailStreak = st.failStreak
	rt.RecoverStreak = st.recoverStreak
	rt.LastProbeAt = now
	rt.LastSwitchAt = st.lastSwitchAt
	rt.ActiveSince = st.activeSince
	if rt.LastSwitchAt.IsZero() && !st.lastSwitchAt.IsZero() {
		rt.LastSwitch = st.lastSwitchAt
	}

	onPrimary := active == sec.PrimaryTag
	onBackup := active == sec.URLTestTag || isBackupTag(active, sec)

	switch {
	case onPrimary:
		st.mode = modePrimary
		if !primaryOK {
			st.failStreak++
			st.recoverStreak = 0
			if st.failStreak >= sec.FailThreshold {
				if err := c.switchTo(ctx, cli, sec, sec.URLTestTag, "primary outage"); err != nil {
					rt.LastError = err.Error()
				} else {
					st.mode = modeBackup
					st.failStreak = 0
				}
			}
		} else {
			st.failStreak = 0
		}

	case onBackup:
		st.mode = modeBackup
		if primaryOK {
			st.recoverStreak++
			st.failStreak = 0
			if st.recoverStreak >= sec.RecoverThreshold {
				if err := c.switchTo(ctx, cli, sec, sec.PrimaryTag, "primary recovered"); err != nil {
					rt.LastError = err.Error()
				} else {
					st.mode = modePrimary
					st.recoverStreak = 0
				}
			}
		} else {
			st.recoverStreak = 0
			// Ensure urltest group is selected among backups.
			if active != sec.URLTestTag {
				_ = c.switchTo(ctx, cli, sec, sec.URLTestTag, "backup urltest")
			}
		}

	default:
		// Manual selection or drift: re-attach to policy.
		if primaryOK {
			_ = c.switchTo(ctx, cli, sec, sec.PrimaryTag, "sync primary")
			st.mode = modePrimary
		} else {
			_ = c.switchTo(ctx, cli, sec, sec.URLTestTag, "sync backup")
			st.mode = modeBackup
		}
	}

	rt.Mode = st.mode
	rt.FailStreak = st.failStreak
	rt.RecoverStreak = st.recoverStreak
	rt.Active = st.lastActive
	return rt
}

func (c *Controller) switchTo(ctx context.Context, cli *clash.Client, sec SectionConfig, target, reason string) error {
	prev := c.stateFor(sec.Section).lastActive
	if prev == target {
		return nil
	}
	if err := cli.SwitchProxy(ctx, sec.SelectorTag, target); err != nil {
		return err
	}
	st := c.stateFor(sec.Section)
	st.lastActive = target
	st.lastSwitchAt = time.Now().UTC()
	st.activeSince = st.lastSwitchAt
	_ = notify.RecordFailover(sec.Section, prev, target, reason)
	ev := notify.Event{
		Time:    time.Now().UTC(),
		Section: sec.Section,
		From:    prev,
		To:      target,
		Reason:  reason,
	}
	_ = notify.SendWebhook(c.Webhook, ev)
	return nil
}

func (c *Controller) recordPassive(sec SectionConfig, st *sectionState, active string) {
	if st.lastActive != "" && st.lastActive != active {
		_ = notify.RecordFailover(sec.Section, st.lastActive, active, "urltest failover")
		ev := notify.Event{
			Time:    time.Now().UTC(),
			Section: sec.Section,
			From:    st.lastActive,
			To:      active,
			Reason:  "urltest failover",
		}
		_ = notify.SendWebhook(c.Webhook, ev)
	}
	st.lastActive = active
}

func (c *Controller) stateFor(section string) *sectionState {
	if c.states == nil {
		c.states = make(map[string]*sectionState)
	}
	st, ok := c.states[section]
	if !ok {
		st = &sectionState{mode: modePrimary}
		c.states[section] = st
	}
	return st
}

func (c *Controller) writeRuntime(r []SectionRuntime) error {
	return writeRuntimeState(r)
}

func loadManagedSections(pkg *uci.Package) []SectionConfig {
	if pkg == nil {
		return nil
	}
	var out []SectionConfig
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		if sec.Get("connection_type", "") != "vpn" {
			continue
		}
		if !sec.GetBool("failover_vpn_enabled", false) {
			continue
		}
		if len(sec.GetList("failover_proxy_links")) == 0 {
			continue
		}
		pol := policy.Normalize(sec.Get("failover_policy", ""))
		backupCount := len(sec.GetList("failover_proxy_links"))
		backups := make([]string, 0, backupCount)
		for i := 1; i <= backupCount; i++ {
			backups = append(backups, singbox.PeerTag(name, i))
		}
		failTh, recTh := policy.Thresholds(sec, pol)
		cfg := SectionConfig{
			Section:          name,
			Policy:           pol,
			PrimaryTag:       singbox.AWGTag(name),
			PrimaryIface:     sec.Get("interface", ""),
			URLTestTag:       singbox.URLTestTag(name),
			SelectorTag:      singbox.OutboundTag(name),
			BackupTags:       backups,
			TestURL:          sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204"),
			FailThreshold:    failTh,
			RecoverThreshold: recTh,
			Sec:              sec,
		}
		out = append(out, cfg)
	}
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil || sec.Get("connection_type", "") != "proxy" {
			continue
		}
		if sec.Get("proxy_config_type", "") != "urltest" {
			continue
		}
		if len(sec.GetList("urltest_proxy_links")) == 0 {
			continue
		}
		backupCount := len(sec.GetList("urltest_proxy_links"))
		backups := make([]string, 0, backupCount)
		for i := 1; i <= backupCount; i++ {
			backups = append(backups, singbox.PeerTag(name, i))
		}
		out = append(out, SectionConfig{
			Section:     name,
			Policy:      policy.Fastest,
			URLTestTag:  singbox.URLTestTag(name),
			SelectorTag: singbox.OutboundTag(name),
			BackupTags:  backups,
			TestURL:     sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204"),
		})
	}
	return out
}

func parseControllerInterval(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(singbox.NormalizeDuration(raw))
	if err != nil || d < 15*time.Second {
		return 30 * time.Second
	}
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}

func probeOutbound(ctx context.Context, cli *clash.Client, sec SectionConfig, tag, testURL string) (delay int, ok bool) {
	bind := sec.PrimaryIface
	if sec.Sec != nil && tag != sec.PrimaryTag {
		bind = probe.BindIfaceForChannel(sec.Section, sec.Sec, tag)
	}
	delay, ok, _ = probe.Outbound(ctx, cli, tag, testURL, "direct", bind)
	return delay, ok
}

func isBackupTag(active string, sec SectionConfig) bool {
	for _, t := range sec.BackupTags {
		if active == t {
			return true
		}
	}
	return false
}

func writeRuntimeState(sections []SectionRuntime) error {
	dir := filepath.Dir(paths.FailoverStateFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload := map[string]any{
		"updated_at": time.Now().UTC(),
		"sections":   sections,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.FailoverStateFile, data, 0o644)
}

// ReadRuntimeState loads last controller snapshot written by the background loop.
func ReadRuntimeState() ([]SectionRuntime, error) {
	data, err := os.ReadFile(paths.FailoverStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var payload struct {
		Sections []SectionRuntime `json:"sections"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload.Sections, nil
}
