package routers_test

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
)

func TestManagerSingleLocalDefault(t *testing.T) {
	mgr, err := routers.NewManager(config.Config{
		ClashAPI:            "http://127.0.0.1:9090",
		RoutingInitScript:   "/etc/init.d/hybrid-failover",
		MainSection:         "main",
		ProbeTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	inst, err := mgr.InstanceFor(42)
	if err != nil {
		t.Fatal(err)
	}
	if inst.ID != "local" {
		t.Fatalf("id: %q", inst.ID)
	}
}

func TestManagerMultiRequiresSelection(t *testing.T) {
	defer routers.SetSeams(func(string) (string, bool) { return "", false }, func(string) bool { return true })()
	mgr, err := routers.NewManager(config.Config{
		Routers: []config.RouterConfig{
			{ID: "a", Name: "A", Local: true},
			{ID: "b", Name: "B", Host: "10.0.0.2", IdentityFile: "/tmp/key"},
		},
		ProbeTimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.InstanceFor(1); err == nil {
		t.Fatal("expected selection error")
	}
	if err := mgr.SetSelected(1, "a"); err != nil {
		t.Fatal(err)
	}
	inst, err := mgr.InstanceFor(1)
	if err != nil || inst.ID != "a" {
		t.Fatalf("instance: %v err=%v", inst, err)
	}
}

func TestManagerSkipsRemoteWithoutKey(t *testing.T) {
	defer routers.SetSeams(func(string) (string, bool) { return "", false }, func(string) bool { return false })()
	mgr, err := routers.NewManager(config.Config{
		Routers: []config.RouterConfig{
			{ID: "local", Name: "Этот роутер", Local: true},
			{ID: "office", Host: "192.168.11.1", IdentityFile: "/etc/hybrid-failover-bot/id_router_office"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mgr.Multi() || len(mgr.Warnings) != 1 {
		t.Fatalf("multi=%v warnings=%v", mgr.Multi(), mgr.Warnings)
	}
	if inst, err := mgr.InstanceFor(7); err != nil || inst.ID != "local" {
		t.Fatalf("local router not auto-selected: %v %v", inst, err)
	}
}

func TestManagerFixesMissingLocalSection(t *testing.T) {
	uci := map[string]string{
		"hybrid-failover.settings":              "settings",
		"hybrid-failover.settings.main_section": "glob",
		"hybrid-failover.glob":                  "section",
	}
	defer routers.SetSeams(func(k string) (string, bool) { v, ok := uci[k]; return v, ok }, func(string) bool { return true })()
	mgr, err := routers.NewManager(config.Config{UCIPackage: "hybrid-failover", MainSection: "main"})
	if err != nil {
		t.Fatal(err)
	}
	inst, _ := mgr.InstanceFor(1)
	if inst.Service.MainSection() != "glob" || len(mgr.Warnings) != 1 {
		t.Fatalf("section=%q warnings=%v", inst.Service.MainSection(), mgr.Warnings)
	}
}
