package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestLoadDomainsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadDomains(plan.RuleSet{Kind: "domains", Path: path})
	if err != nil {
		t.Fatalf("empty ruleset must not fail engine start: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestLoadDomainsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadDomains(plan.RuleSet{Kind: "domains", Path: path})
	if err != nil {
		t.Fatalf("invalid ruleset must not fail engine start: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestLoadDomainsValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	body := `{"version":3,"rules":[{"domain_suffix":["t.me","telegram.org"]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadDomains(plan.RuleSet{Kind: "domains", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "t.me" || got[1] != "telegram.org" {
		t.Fatalf("got %v", got)
	}
}
