package singbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSourceRulesetRepairsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glob-russia_inside-community-ruleset.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	EnsureSourceRuleset(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || !strings.Contains(string(data), `"rules"`) {
		t.Fatalf("stub not written: %q", data)
	}
	if DomainRulesetEmpty(path) != true {
		t.Fatal("empty stub should still count as empty for list-update")
	}
}

func TestWriteDomainRulesetAtomicValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDomainRuleset(path, []string{"t.me"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || !strings.Contains(string(data), "t.me") {
		t.Fatalf("got %q", data)
	}
}
