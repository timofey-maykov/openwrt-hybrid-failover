package policy

import (
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// VPNFailover policy for hybrid-failover UCI option failover_policy.
type VPNFailover string

const (
	OutageOnly     VPNFailover = "outage-only"
	PreferPrimary  VPNFailover = "prefer-primary"
	Fastest        VPNFailover = "fastest"
)

// Normalize maps UCI failover_policy (default outage-only).
func Normalize(raw string) VPNFailover {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(Fastest), "latency", "urltest":
		return Fastest
	case string(PreferPrimary):
		return PreferPrimary
	default:
		return OutageOnly
	}
}

func (p VPNFailover) Managed() bool {
	return p == OutageOnly || p == PreferPrimary
}

func (p VPNFailover) FailThreshold() int {
	return 2
}

func (p VPNFailover) RecoverThreshold() int {
	if p == PreferPrimary {
		return 1
	}
	return 2
}

// Thresholds returns fail/recover streak limits; UCI overrides policy defaults.
func Thresholds(sec *uci.Section, pol VPNFailover) (fail, recover int) {
	fail = pol.FailThreshold()
	recover = pol.RecoverThreshold()
	if sec == nil {
		return fail, recover
	}
	if v := strings.TrimSpace(sec.Get("failover_fail_threshold", "")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			fail = n
		}
	}
	if v := strings.TrimSpace(sec.Get("failover_recover_threshold", "")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			recover = n
		}
	}
	return fail, recover
}
