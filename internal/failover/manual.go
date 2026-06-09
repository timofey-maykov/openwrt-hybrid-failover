package failover

import (
	"fmt"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// SelectorMembers returns outbound tags allowed for manual switch on a section.
func SelectorMembers(pkg *uci.Package, section string) ([]string, error) {
	if pkg == nil {
		return nil, fmt.Errorf("uci package required")
	}
	sec := pkg.Section(section)
	if sec == nil {
		return nil, fmt.Errorf("section %q not found", section)
	}
	conn := sec.Get("connection_type", "")
	switch conn {
	case "vpn":
		if !sec.GetBool("failover_vpn_enabled", false) {
			return nil, fmt.Errorf("failover not enabled for section %q", section)
		}
		links := sec.GetList("failover_proxy_links")
		if len(links) == 0 {
			return nil, fmt.Errorf("no failover proxies configured")
		}
		pol := sec.Get("failover_policy", "")
		members := []string{singbox.AWGTag(section), singbox.URLTestTag(section)}
		for i := range links {
			members = append(members, singbox.PeerTag(section, i+1))
		}
		if strings.EqualFold(pol, "fastest") || pol == "latency" || pol == "urltest" {
			return []string{singbox.OutboundTag(section)}, nil
		}
		return members, nil
	case "proxy":
		if sec.Get("proxy_config_type", "") != "urltest" {
			return nil, fmt.Errorf("section %q is not urltest proxy", section)
		}
		links := sec.GetList("urltest_proxy_links")
		members := []string{singbox.URLTestTag(section)}
		for i := range links {
			members = append(members, singbox.PeerTag(section, i+1))
		}
		return members, nil
	default:
		return nil, fmt.Errorf("unsupported section type for switch")
	}
}

// ValidateSelectorMember reports whether outbound is a member of the section selector.
func ValidateSelectorMember(pkg *uci.Package, section, outbound string) error {
	members, err := SelectorMembers(pkg, section)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m == outbound {
			return nil
		}
	}
	return fmt.Errorf("outbound %q is not a member of selector for section %q", outbound, section)
}

// NoteManualSwitch persists controller streak reset after operator-driven switch.
func NoteManualSwitch(section, outbound string) error {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return err
	}
	if err := ValidateSelectorMember(pkg, section, outbound); err != nil {
		return err
	}

	primaryTag := singbox.AWGTag(section)
	urlTestTag := singbox.URLTestTag(section)
	mode := modeBackup
	if outbound == primaryTag {
		mode = modePrimary
	} else if outbound == urlTestTag {
		mode = modeBackup
	}

	now := time.Now().UTC()
	states, _ := ReadRuntimeState()
	found := false
	pol := ""
	if sec := pkg.Section(section); sec != nil {
		pol = sec.Get("failover_policy", "")
	}
	for i := range states {
		if states[i].Section != section {
			continue
		}
		states[i].Active = outbound
		states[i].Mode = mode
		states[i].FailStreak = 0
		states[i].RecoverStreak = 0
		states[i].LastSwitchAt = now
		states[i].ActiveSince = now
		states[i].LastSwitch = now
		states[i].Policy = pol
		found = true
		break
	}
	if !found {
		states = append(states, SectionRuntime{
			Section:       section,
			Policy:        pol,
			Mode:          mode,
			Active:        outbound,
			FailStreak:    0,
			RecoverStreak: 0,
			LastSwitchAt:  now,
			ActiveSince:   now,
			LastSwitch:    now,
		})
	}
	return writeRuntimeState(states)
}

func (c *Controller) syncStatesFromDisk() {
	runtimes, err := ReadRuntimeState()
	if err != nil || len(runtimes) == 0 {
		return
	}
	for _, rt := range runtimes {
		st := c.stateFor(rt.Section)
		if rt.LastSwitchAt.IsZero() {
			continue
		}
		if st.lastSwitchAt.IsZero() || rt.LastSwitchAt.After(st.lastSwitchAt) {
			if rt.Mode != "" {
				st.mode = rt.Mode
			}
			st.failStreak = rt.FailStreak
			st.recoverStreak = rt.RecoverStreak
			if rt.Active != "" {
				st.lastActive = rt.Active
			}
			st.lastSwitchAt = rt.LastSwitchAt
			if !rt.ActiveSince.IsZero() {
				st.activeSince = rt.ActiveSince
			}
		}
	}
}

func (c *Controller) hydrateStatesOnStart() {
	runtimes, err := ReadRuntimeState()
	if err != nil {
		return
	}
	for _, rt := range runtimes {
		st := c.stateFor(rt.Section)
		if rt.Mode != "" {
			st.mode = rt.Mode
		}
		st.failStreak = rt.FailStreak
		st.recoverStreak = rt.RecoverStreak
		if rt.Active != "" {
			st.lastActive = rt.Active
		}
		if !rt.LastSwitchAt.IsZero() {
			st.lastSwitchAt = rt.LastSwitchAt
		}
		if !rt.ActiveSince.IsZero() {
			st.activeSince = rt.ActiveSince
		} else if !rt.LastSwitchAt.IsZero() {
			st.activeSince = rt.LastSwitchAt
		}
	}
}

func (c *Controller) reloadSettings(pkg *uci.Package) {
	if pkg == nil {
		return
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return
	}
	c.ClashURL = clash.ResolveBaseURL(settings)
	c.Webhook = settings.Get("webhook_url", "")
	if c.Webhook == "" {
		c.Webhook = settings.Get("failover_webhook_url", "")
	}
	c.Interval = parseControllerInterval(settings.Get("failover_probe_interval", ""))
}
