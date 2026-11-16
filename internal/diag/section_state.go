package diag

import "github.com/tmaykov/openwrt-hybrid-failover/internal/uci"

// WithSectionState removes stale runtime data for a deliberately disabled
// section. The engine can still be serving other enabled sections.
func WithSectionState(r Report, section string, sec *uci.Section) Report {
	r.SectionDisabled = sec != nil && !sec.GetBool("enabled", true)
	if !r.SectionDisabled {
		return r
	}
	r.ActiveOutbound = ""
	r.Channels = nil
	r.Failover = &FailoverInfo{Section: section}
	controllers := make([]ControllerSection, 0, len(r.Controller))
	for _, c := range r.Controller {
		if c.Section != section {
			controllers = append(controllers, c)
		}
	}
	r.Controller = controllers
	return r
}
