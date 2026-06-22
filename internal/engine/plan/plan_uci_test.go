package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestCompilePlanFromExamples(t *testing.T) {
	root := filepath.Join("..", "..", "..", "examples", "testdata")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skip("examples/testdata not found:", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		path := filepath.Join(root, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			pkg, err := uci.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			p, err := plan.CompilePlan(pkg)
			if err != nil {
				t.Fatal(err)
			}
			if err := plan.ValidatePlan(p); err != nil {
				t.Fatal(err)
			}
			if len(p.Outbounds) == 0 {
				t.Fatal("expected outbounds")
			}
		})
	}
}

func TestEngineModeDefault(t *testing.T) {
	pkg := &uci.Package{}
	if plan.EngineMode(pkg) != plan.ModeNative {
		t.Fatalf("nil package should default to native")
	}
}
