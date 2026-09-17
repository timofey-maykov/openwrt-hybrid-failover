package diag

import (
	"context"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"net"
	"testing"
	"time"
)

func TestHealthRejectsSilentTCPServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		<-done
	}()
	reg, err := outbound.NewRegistry([]plan.OutboundPlan{{Tag: "silent", Kind: plan.OutboundDirect}})
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	delay, ok, detail := probeRegistryOutbound(ctx, reg, "silent", "https://"+ln.Addr().String()+"/generate_204")
	if ok {
		t.Fatalf("health reported silent non-TLS/non-HTTP server as alive: delay=%d detail=%q", delay, detail)
	}
}
