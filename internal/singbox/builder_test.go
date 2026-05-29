package singbox_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(root, "examples", "testdata", name)
}

func buildFromFixture(t *testing.T, fixture string) (*singbox.Config, []byte) {
	t.Helper()
	pkg, err := uci.Load(testdataPath(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := singbox.NewBuilder(pkg).Build()
	if err != nil {
		t.Fatal(err)
	}
	data, err := cfg.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return cfg, data
}

func hasOutboundTag(outbounds []map[string]any, tag string) bool {
	for _, ob := range outbounds {
		if ob["tag"] == tag {
			return true
		}
	}
	return false
}

func hasOutboundType(outbounds []map[string]any, typ string) bool {
	for _, ob := range outbounds {
		if ob["type"] == typ {
			return true
		}
	}
	return false
}

func routeRules(cfg *singbox.Config) []map[string]any {
	rules, _ := cfg.Route["rules"].([]map[string]any)
	return rules
}

func hasRejectRouteRule(cfg *singbox.Config) bool {
	for _, rule := range routeRules(cfg) {
		if rule["action"] == "reject" {
			return true
		}
	}
	return false
}

func TestBuilderVPNFailoverGolden(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	cfg, data := buildFromFixture(t, "hybrid-failover-vpn-failover.conf")

	golden := filepath.Join(filepath.Dir(thisFile), "testdata", "glob-vpn-failover.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden file (run UPDATE_GOLDEN=1 go test): %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("config mismatch:\n%s", data)
	}

	if len(cfg.Outbounds) < 4 {
		t.Fatalf("expected at least 4 outbounds, got %d", len(cfg.Outbounds))
	}
	if !hasOutboundType(cfg.Outbounds, "urltest") {
		t.Fatal("missing urltest outbound")
	}
}

func TestBuilderStructuralFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		check    func(t *testing.T, cfg *singbox.Config)
	}{
		{
			name:    "proxy urltest section",
			fixture: "proxy-urltest.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasOutboundType(cfg.Outbounds, "urltest") {
					t.Fatal("missing urltest outbound")
				}
				if !hasOutboundTag(cfg.Outbounds, "proxytest-out") {
					t.Fatal("missing selector tag proxytest-out")
				}
				if !hasOutboundTag(cfg.Outbounds, "proxytest-urltest-out") {
					t.Fatal("missing urltest tag proxytest-urltest-out")
				}
			},
		},
		{
			name:    "proxy outbound_json",
			fixture: "outbound-json.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasOutboundTag(cfg.Outbounds, "custom-out") {
					t.Fatal("missing outbound_json tag custom-out")
				}
				found := false
				for _, ob := range cfg.Outbounds {
					if ob["tag"] == "custom-out" && ob["type"] == "socks" {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("custom-out is not a socks outbound")
				}
			},
		},
		{
			name:    "block with community_lists twitter",
			fixture: "block-section.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasRejectRouteRule(cfg) {
					t.Fatal("missing block reject route rule")
				}
				if len(cfg.RuleSets) == 0 {
					t.Fatal("expected community ruleset for twitter")
				}
			},
		},
		{
			name:    "vpn domain resolver",
			fixture: "vpn-domain-resolver.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasOutboundTag(cfg.Outbounds, "vpndr-out") {
					t.Fatal("missing vpndr-out")
				}
				for _, ob := range cfg.Outbounds {
					if ob["tag"] == "vpndr-out" {
						if ob["domain_resolver"] != "vpndr-domain-resolver" {
							t.Fatalf("vpndr-out domain_resolver = %v, want vpndr-domain-resolver", ob["domain_resolver"])
						}
					}
				}
				servers, _ := cfg.DNS["servers"].([]map[string]any)
				foundResolver := false
				for _, srv := range servers {
					if srv["tag"] == "vpndr-domain-resolver" {
						foundResolver = true
						if srv["detour"] != "vpndr-out" {
							t.Fatalf("domain resolver detour = %v, want vpndr-out", srv["detour"])
						}
					}
				}
				if !foundResolver {
					t.Fatal("missing vpndr-domain-resolver DNS server")
				}
			},
		},
		{
			name:    "community lists multi-service",
			fixture: "community-lists.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasRejectRouteRule(cfg) {
					t.Fatal("missing block reject route rule")
				}
				if len(cfg.RuleSets) < 2 {
					t.Fatalf("expected at least 2 community rulesets, got %d", len(cfg.RuleSets))
				}
			},
		},
		{
			name:    "multi-section glob vpn and proxy",
			fixture: "multi-section.conf",
			check: func(t *testing.T, cfg *singbox.Config) {
				if !hasOutboundTag(cfg.Outbounds, "glob-out") {
					t.Fatal("missing glob-out selector")
				}
				if !hasOutboundTag(cfg.Outbounds, "directproxy-out") {
					t.Fatal("missing directproxy-out")
				}
				if !hasOutboundType(cfg.Outbounds, "urltest") {
					t.Fatal("missing urltest for vpn failover section")
				}
				proxyCount := 0
				for _, ob := range cfg.Outbounds {
					if typ, _ := ob["type"].(string); typ == "vless" || typ == "trojan" || typ == "shadowsocks" {
						proxyCount++
					}
				}
				if proxyCount < 2 {
					t.Fatalf("expected proxy outbounds from both sections, got %d", proxyCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := buildFromFixture(t, tt.fixture)
			tt.check(t, cfg)
		})
	}
}

func TestBuilderSingboxCheckOptional(t *testing.T) {
	if os.Getenv("HF_SINGBOX_CHECK") != "1" {
		t.Skip("skipped (set HF_SINGBOX_CHECK=1 to run sing-box check on fixtures)")
	}
	if _, err := exec.LookPath("sing-box"); err != nil {
		t.Skip("sing-box not installed")
	}
	fixtures := []string{
		"hybrid-failover-vpn-failover.conf",
		"proxy-urltest.conf",
		"outbound-json.conf",
		"multi-section.conf",
		"block-section.conf",
		"vpn-domain-resolver.conf",
		"community-lists.conf",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			_, data := buildFromFixture(t, fixture)
			tmp := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(tmp, data, 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("sing-box", "check", "-c", tmp).CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed: %v: %s", err, string(out))
			}
		})
	}
}

func TestBuilderJSONRoundTrip(t *testing.T) {
	cfg, data := buildFromFixture(t, "proxy-urltest.conf")
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["outbounds"] == nil {
		t.Fatal("missing outbounds key")
	}
	_ = cfg
}
