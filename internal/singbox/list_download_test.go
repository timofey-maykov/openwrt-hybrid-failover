package singbox

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestListDownloadSectionUsesMainSection(t *testing.T) {
	pkg, err := uci.Parse("" +
		"config settings 'settings'\n" +
		"\toption main_section 'glob'\n" +
		"\toption download_lists_via_proxy '1'\n")
	if err != nil {
		t.Fatal(err)
	}
	enabled, section := ListDownloadSection(pkg)
	if !enabled || section != "glob" {
		t.Fatalf("enabled=%v section=%q", enabled, section)
	}
}

func TestListDownloadSectionDisabled(t *testing.T) {
	pkg, err := uci.Parse("config settings 'settings'\n")
	if err != nil {
		t.Fatal(err)
	}
	enabled, section := ListDownloadSection(pkg)
	if enabled || section != "" {
		t.Fatalf("enabled=%v section=%q", enabled, section)
	}
}
