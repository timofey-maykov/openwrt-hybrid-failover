package policy

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestThresholdsUCIOverride(t *testing.T) {
	sec := &uci.Section{Options: map[string]string{
		"failover_fail_threshold":    "3",
		"failover_recover_threshold": "1",
	}}
	fail, recover := Thresholds(sec, OutageOnly)
	if fail != 3 || recover != 1 {
		t.Fatalf("got fail=%d recover=%d", fail, recover)
	}
}

func TestThresholdsDefaults(t *testing.T) {
	fail, recover := Thresholds(nil, PreferPrimary)
	if fail != 2 || recover != 1 {
		t.Fatalf("got fail=%d recover=%d", fail, recover)
	}
}
