package failover

import "github.com/tmaykov/openwrt-hybrid-failover/internal/policy"

// DryRunHint describes controller intent without switching.
type DryRunHint struct {
	Section    string `json:"section"`
	Suggestion string `json:"suggestion"`
}

// BuildDryRunHints summarizes next likely controller action per section.
func BuildDryRunHints(states []SectionRuntime) []DryRunHint {
	var hints []DryRunHint
	for _, st := range states {
		if st.Policy == string(policy.Fastest) || st.Policy == "" {
			continue
		}
		h := DryRunHint{Section: st.Section}
		switch {
		case st.Mode == "primary" && !st.PrimaryOK:
			h.Suggestion = "primary failing; switch to backup after fail threshold"
		case st.Mode == "backup" && st.PrimaryOK:
			h.Suggestion = "primary OK; switch back after recover threshold"
		case st.Mode == "primary" && st.PrimaryOK:
			h.Suggestion = "stay on primary VPN"
		default:
			h.Suggestion = "on backup; primary still down"
		}
		hints = append(hints, h)
	}
	return hints
}
