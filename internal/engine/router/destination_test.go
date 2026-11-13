package router

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestSniffedCamouflageSNIDoesNotReplaceRealVPNServer(t *testing.T) {
	r, err := New(&plan.Plan{DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ip, domain, want string
		port             int
	}{
		{"13.140.81.153", "www.bing.com", "13.140.81.153:9446", 9446},
		{"2001:db8::1", "www.bing.com", "[2001:db8::1]:9446", 9446},
		{"", "origin.example", "origin.example:443", 443},
		{"198.18.1.250", "origin.example", "origin.example:443", 443},
	} {
		if got := r.dialTarget(plan.ConnMeta{DstIP: tc.ip, Domain: tc.domain, DstPort: tc.port}); got != tc.want {
			t.Errorf("target %s, want %s", got, tc.want)
		}
	}
	fake, ok := r.ResolveFakeIP("original.example")
	if !ok {
		t.Fatal("cannot allocate fake IP")
	}
	if got := r.dialTarget(plan.ConnMeta{DstIP: fake, Domain: "cover.example", DstPort: 443}); got != "original.example:443" {
		t.Fatalf("FakeIP mapping lost: %s", got)
	}
}

func TestDialKeepsClientIPDestinationWithDifferentSniffedName(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			defer conn.Close()
			_, _ = conn.Write([]byte("original destination"))
		}
	}()
	p := &plan.Plan{DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange}, Outbounds: []plan.OutboundPlan{{Tag: plan.DirectTag, Kind: plan.OutboundDirect}}}
	registry, err := outbound.NewRegistry(context.Background(), p.Outbounds)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Stop()
	r, err := New(p, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := r.Dial(plan.ConnMeta{DstIP: "127.0.0.1", Domain: "127.0.0.2", DstPort: listener.Addr().(*net.TCPAddr).Port, Network: "tcp"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	body, err := io.ReadAll(conn)
	if err != nil || string(body) != "original destination" {
		t.Fatalf("wrong destination: %q %v", body, err)
	}
}
