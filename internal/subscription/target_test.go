package subscription

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionTargetMustUseItsImportedChannels(t *testing.T) {
	for _, tc := range []struct {
		conn, kind, backup string
		ok                 bool
	}{
		{"vpn", "", "0", false}, {"vpn", "", "1", true}, {"proxy", "urltest", "", true}, {"proxy", "url", "", false}, {"block", "", "", false},
	} {
		sec := &uci.Section{Options: map[string]string{"connection_type": tc.conn, "proxy_config_type": tc.kind, "failover_vpn_enabled": tc.backup}}
		if err := ValidateTarget(sec); (err == nil) != tc.ok {
			t.Fatalf("mode=%s kind=%s backup=%s error=%v", tc.conn, tc.kind, tc.backup, err)
		}
	}
}

func TestInactiveVPNImportFailsBeforeWritingUCI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	conf := "config section 'glob'\n option connection_type 'vpn'\n option interface 'vpn0'\n option failover_vpn_enabled '0'\n"
	if err := os.WriteFile(path, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := ApplyToUCI(path, "glob", []string{"hy2://password@192.0.2.1:443"}, true)
	if err == nil || !strings.Contains(err.Error(), "enable backup channels") {
		t.Fatalf("error=%v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != conf {
		t.Fatal("inactive VPN config changed")
	}
}
