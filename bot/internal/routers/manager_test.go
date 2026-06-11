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
