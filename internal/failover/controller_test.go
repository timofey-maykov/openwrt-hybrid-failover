package failover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/policy"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestValidateSelectorMember(t *testing.T) {
	pkg, err := uci.Parse(`
config section 'glob'
	option connection_type 'vpn'
	option failover_vpn_enabled '1'
	list failover_proxy_links 'vless://x@1.2.3.4:443'
`)
	if err != nil {
		t.Fatal(err)
	}
	members, err := SelectorMembers(pkg, "glob")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) < 3 {
		t.Fatalf("members: %v", members)
	}
	if err := ValidateSelectorMember(pkg, "glob", singbox.AWGTag("glob")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSelectorMember(pkg, "glob", "evil-out"); err == nil {
		t.Fatal("expected reject unknown outbound")
	}
}

func TestNoteManualSwitchResetsStreaks(t *testing.T) {
	dir := t.TempDir()
	uciPath := filepath.Join(dir, "hybrid-failover")
	statePath := filepath.Join(dir, "policy-state.json")
	oldUCI, oldState := paths.UCIConfig, paths.FailoverStateFile
	paths.UCIConfig = uciPath
	paths.FailoverStateFile = statePath
	t.Cleanup(func() {
		paths.UCIConfig = oldUCI
		paths.FailoverStateFile = oldState
	})

	if err := os.WriteFile(uciPath, []byte(`
config section 'glob'
	option connection_type 'vpn'
	option failover_vpn_enabled '1'
	option failover_policy 'outage-only'
	list failover_proxy_links 'vless://x@1.2.3.4:443'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = writeRuntimeState([]SectionRuntime{{
		Section:       "glob",
		Policy:        "outage-only",
		Mode:          "primary",
		Active:        singbox.AWGTag("glob"),
		FailStreak:    5,
		RecoverStreak: 0,
	}})

	if err := NoteManualSwitch("glob", singbox.URLTestTag("glob")); err != nil {
		t.Fatal(err)
	}
	states, err := ReadRuntimeState()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("states: %+v", states)
	}
	st := states[0]
	if st.FailStreak != 0 || st.RecoverStreak != 0 {
		t.Fatalf("streaks not reset: %+v", st)
	}
	if st.Active != singbox.URLTestTag("glob") || st.Mode != modeBackup {
		t.Fatalf("active/mode: %+v", st)
	}
}

func TestPollSectionPrimaryFailover(t *testing.T) {
	selector := singbox.OutboundTag("glob")
	primary := singbox.AWGTag("glob")
	urltest := singbox.URLTestTag("glob")

	var active = primary
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/proxies":
			_ = json.NewEncoder(w).Encode(clash.ProxiesResponse{
				Proxies: map[string]clash.ProxyInfo{
					selector: {Name: selector, Type: "selector", Now: active},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/proxies/"+primary+"/delay":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"timeout"}`))
		case r.Method == http.MethodPut:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			active = body["name"]
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Controller{ClashURL: srv.URL, states: make(map[string]*sectionState)}
	sec := SectionConfig{
		Section:          "glob",
		Policy:           policy.OutageOnly,
		PrimaryTag:       primary,
		URLTestTag:       urltest,
		SelectorTag:      selector,
		TestURL:          "https://example.com/generate_204",
		FailThreshold:    2,
		RecoverThreshold: 2,
	}
	cli := clash.New(srv.URL, 5*time.Second)
	ctx := context.Background()

	c.pollSection(ctx, cli, sec, nil)
	c.pollSection(ctx, cli, sec, nil)
	rt := c.pollSection(ctx, cli, sec, nil)
	if rt.Active != urltest && c.stateFor("glob").lastActive != urltest {
		t.Fatalf("after 2 fails want urltest active, got rt=%q mem=%q", rt.Active, c.stateFor("glob").lastActive)
	}
}

func TestSyncStatesFromDisk(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "policy-state.json")
	oldState := paths.FailoverStateFile
	paths.FailoverStateFile = statePath
	t.Cleanup(func() {
		paths.FailoverStateFile = oldState
	})

	now := time.Now().UTC()
	_ = writeRuntimeState([]SectionRuntime{{
		Section:       "glob",
		Mode:          modeBackup,
		Active:        singbox.URLTestTag("glob"),
		FailStreak:    0,
		RecoverStreak: 0,
		LastSwitchAt:  now,
		ActiveSince:   now,
	}})

	c := &Controller{states: make(map[string]*sectionState)}
	st := c.stateFor("glob")
	st.failStreak = 3
	c.syncStatesFromDisk()
	if st.failStreak != 0 {
		t.Fatalf("fail streak: got %d want 0", st.failStreak)
	}
	if st.mode != modeBackup {
		t.Fatalf("mode: %q", st.mode)
	}
}
