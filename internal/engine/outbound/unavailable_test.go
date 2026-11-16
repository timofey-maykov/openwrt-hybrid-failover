package outbound

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestSubscription98FingerprintsBuildTogether(t *testing.T) {
	var plans []plan.OutboundPlan
	for i := 0; i < 98; i++ {
		fp := fingerprintNames[i%len(fingerprintNames)]
		if i == 63 {
			fp = "qq"
		}
		plans = append(plans, plan.OutboundPlan{Tag: fmt.Sprintf("glob-%d-out", i), Kind: plan.OutboundVLESS, ProxyURI: "vless://11111111-1111-4111-8111-111111111111@192.0.2.1:443?security=tls&fp=" + fp})
	}
	reg, err := NewRegistry(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()
	if len(reg.failed) != 0 || len(reg.handlers) != 98 {
		t.Fatalf("handlers=%d failed=%v", len(reg.handlers), reg.failed)
	}
}

func TestBadServerDoesNotStopRegistryOrLeakTraffic(t *testing.T) {
	ctx := context.Background()
	reg, err := NewRegistry(ctx, []plan.OutboundPlan{
		{Tag: "bad", Kind: plan.OutboundVLESS, ProxyURI: "vless://11111111-1111-4111-8111-111111111111@192.0.2.1:443?security=tls&fp=not-supported"},
		{Tag: "qq", Kind: plan.OutboundVLESS, ProxyURI: "vless://11111111-1111-4111-8111-111111111111@192.0.2.1:443?security=tls&fp=qq"},
		{Tag: "good", Kind: plan.OutboundDirect},
		{Tag: "pool", Kind: plan.OutboundURLTest, Members: []string{"bad", "good"}},
		{Tag: "broken-pool", Kind: plan.OutboundURLTest, Members: []string{"bad"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()
	ctrl := control.New()
	if err := reg.StartURLTests(ctx, ctrl); err != nil {
		t.Fatal(err)
	}
	if sample := ctrl.Delay("bad"); sample.OK || !strings.Contains(sample.Error, "not-supported") {
		t.Fatalf("missing channel diagnostic: %+v", sample)
	}
	if len(reg.failed) != 1 || reg.failed["bad"] == nil {
		t.Fatalf("unexpected failures: %v", reg.failed)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "healthy") }))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	for _, tag := range []string{"bad", "broken-pool"} {
		if c, err := reg.DialTCP(ctx, tag, "tcp", address); err == nil || c != nil {
			t.Fatalf("%s leaked traffic", tag)
		}
		if c, err := reg.DialUDP(ctx, tag, "udp", address); err == nil || c != nil {
			t.Fatalf("%s leaked UDP", tag)
		}
	}
	c, err := reg.DialTCP(ctx, "pool", "tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}
