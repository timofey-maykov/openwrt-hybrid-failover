package pending

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/lifecycle"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func TestSaveChangesMerge(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.SaveChanges(map[string]string{
		"hybrid-failover.glob.urltest_tolerance": "50",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChanges(map[string]string{
		"hybrid-failover.glob.urltest_check_interval": "30s",
	}); err != nil {
		t.Fatal(err)
	}

	snap, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Changes["hybrid-failover.glob.urltest_tolerance"] != "50" {
		t.Fatalf("tolerance: got %q", snap.Changes["hybrid-failover.glob.urltest_tolerance"])
	}
	if snap.Changes["hybrid-failover.glob.urltest_check_interval"] != "30s" {
		t.Fatalf("interval: got %q", snap.Changes["hybrid-failover.glob.urltest_check_interval"])
	}
}

func TestValidateURLTestPair(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Save(map[string]string{
		"hybrid-failover.glob.urltest_check_interval": "5m",
		"hybrid-failover.glob.urltest_idle_timeout":   "30s",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Validate(); err == nil {
		t.Fatal("expected validation error for interval > idle")
	}
}

func TestValidateNoPending(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Validate(); err != nil {
		t.Fatalf("missing pending should validate ok: %v", err)
	}
}

func TestRollbackNoPending(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Rollback(); err != nil {
		t.Fatalf("rollback without pending: %v", err)
	}
}

func TestValidateOK(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Save(map[string]string{
		"hybrid-failover.glob.urltest_check_interval": "30s",
		"hybrid-failover.glob.urltest_idle_timeout":   "5m",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackRemovesSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Save(map[string]string{"hybrid-failover.glob.enabled": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !os.IsNotExist(err) {
		t.Fatalf("expected missing pending file, got %v", err)
	}
}

func TestParseUCIChangesOutput(t *testing.T) {
	changes, err := ParseUCIChangesOutput(stringsJoinLines(
		"hybrid-failover.glob.urltest_tolerance='50'",
		"hybrid-failover.glob.failover_proxy_links+='vless://example'",
		"-hybrid-failover.glob.stale_option",
	))
	if err != nil {
		t.Fatal(err)
	}
	if changes["hybrid-failover.glob.urltest_tolerance"] != "50" {
		t.Fatalf("set: %q", changes["hybrid-failover.glob.urltest_tolerance"])
	}
	if changes["hybrid-failover.glob.failover_proxy_links"] != opAddListPrefix+"vless://example" {
		t.Fatalf("add_list: %q", changes["hybrid-failover.glob.failover_proxy_links"])
	}
	if changes["hybrid-failover.glob.stale_option"] != opDelete {
		t.Fatalf("delete: %q", changes["hybrid-failover.glob.stale_option"])
	}

	delChanges, err := ParseUCIChangesOutput("hybrid-failover.glob.failover_proxy_links-='vless://old'")
	if err != nil {
		t.Fatal(err)
	}
	if delChanges["hybrid-failover.glob.failover_proxy_links"] != opDelListPrefix+"vless://old" {
		t.Fatalf("del_list: %q", delChanges["hybrid-failover.glob.failover_proxy_links"])
	}
}

func TestParseUCIChangesQuotedEscapes(t *testing.T) {
	changes, err := ParseUCIChangesOutput("hybrid-failover.glob.note='it''s fine'")
	if err != nil {
		t.Fatal(err)
	}
	if changes["hybrid-failover.glob.note"] != "it's fine" {
		t.Fatalf("got %q", changes["hybrid-failover.glob.note"])
	}
}

func stringsJoinLines(lines ...string) string {
	var b string
	for i, l := range lines {
		if i > 0 {
			b += "\n"
		}
		b += l
	}
	return b
}

func TestDefaultPendingPath(t *testing.T) {
	if paths.PendingDir == "" {
		t.Fatal("paths.PendingDir unset")
	}
	_ = filepath.Join(paths.PendingDir, "pending.json")
}

func fakeUCI(t *testing.T, changes string) *[]string {
	t.Helper()
	var calls []string
	origExec, origReload := execUCI, applyAndReload
	t.Cleanup(func() { execUCI, applyAndReload = origExec, origReload })
	execUCI = func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == "changes" {
			return changes, nil
		}
		return "", nil
	}
	applyAndReload = func(lifecycle.Options) (lifecycle.Result, error) { return lifecycle.Result{}, nil }
	return &calls
}

func TestApplyCommitsStagedChangesWithoutReplay(t *testing.T) {
	st := NewStore(t.TempDir())
	if err := st.Save(map[string]string{
		"hybrid-failover.glob.failover_proxy_links": opAddListPrefix + "vless://b",
		"hybrid-failover.glob.urltest_tolerance":    opDelete,
	}); err != nil {
		t.Fatal(err)
	}
	calls := fakeUCI(t, "hybrid-failover.glob.failover_proxy_links+='vless://b'\n-hybrid-failover.glob.urltest_tolerance\n")
	if err := st.ApplyViaUCI(); err != nil {
		t.Fatal(err)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "add_list") || strings.HasPrefix(c, "delete") {
			t.Fatalf("staged change replayed: %q (calls %v)", c, *calls)
		}
	}
	if (*calls)[len(*calls)-1] != "commit hybrid-failover" {
		t.Fatalf("expected commit last, got %v", *calls)
	}
}

func TestApplyReplaysWhenStagingLost(t *testing.T) {
	st := NewStore(t.TempDir())
	if err := st.Save(map[string]string{
		"hybrid-failover.glob.failover_proxy_links": opAddListPrefix + "vless://b",
	}); err != nil {
		t.Fatal(err)
	}
	calls := fakeUCI(t, "")
	if err := st.ApplyViaUCI(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "|")
	if !strings.Contains(joined, "add_list hybrid-failover.glob.failover_proxy_links=vless://b") {
		t.Fatalf("snapshot not replayed: %v", *calls)
	}
}
