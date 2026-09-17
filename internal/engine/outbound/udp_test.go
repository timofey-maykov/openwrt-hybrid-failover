package outbound

import (
	"context"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"io"
	"net"
	"testing"
	"time"
)

func TestDirectUDPExchangeIgnoresFakeIP(t *testing.T) {
	server, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() {
		b := make([]byte, 64)
		n, a, e := server.ReadFrom(b)
		if e == nil {
			server.WriteTo(b[:n], a)
		}
	}()
	for _, kind := range []plan.OutboundKind{plan.OutboundAWG2Bind} {
		reg, err := NewRegistry([]plan.OutboundPlan{{Tag: "udp", Kind: kind}})
		if err != nil {
			t.Fatal(err)
		}
		defer reg.Stop()
		pc, err := reg.DialUDP(context.Background(), "udp", "udp", server.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer pc.Close()
		pc.SetDeadline(time.Now().Add(time.Second))
		if _, err = pc.WriteTo([]byte("echo"), &net.UDPAddr{IP: net.ParseIP("198.18.1.1"), Port: 9999}); err != nil {
			t.Fatal(err)
		}
		b := make([]byte, 64)
		n, _, err := pc.ReadFrom(b)
		if err != nil || string(b[:n]) != "echo" {
			t.Fatalf("reply=%q error=%v", b[:n], err)
		}
	}
}

type recordingPacketConn struct{ dest net.Addr }

func (p *recordingPacketConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, io.EOF }
func (p *recordingPacketConn) WriteTo(b []byte, a net.Addr) (int, error) {
	p.dest = a
	return len(b), nil
}
func (p *recordingPacketConn) Close() error                     { return nil }
func (p *recordingPacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (p *recordingPacketConn) SetDeadline(time.Time) error      { return nil }
func (p *recordingPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (p *recordingPacketConn) SetWriteDeadline(time.Time) error { return nil }

func TestBoundPacketPreservesDomain(t *testing.T) {
	pc := &recordingPacketConn{}
	c := &boundPacketConn{conn: pc, dest: parseDestAddr("example.test:3478")}
	if _, err := c.WriteTo([]byte("packet"), &net.UDPAddr{IP: net.ParseIP("198.18.1.1"), Port: 3478}); err != nil {
		t.Fatal(err)
	}
	if pc.dest.String() != "example.test:3478" {
		t.Fatalf("destination=%s", pc.dest)
	}
}

func TestNumericProxyFields(t *testing.T) {
	for _, v := range []any{8443, float64(8443)} {
		if got := numericField(v); got != 8443 {
			t.Fatalf("got %d", got)
		}
	}
	h, err := newVLESSHandler(plan.OutboundPlan{Tag: "v", ProxyURI: "vless://11111111-1111-1111-1111-111111111111@127.0.0.1:8443"})
	if err != nil {
		t.Fatal(err)
	}
	if h.(*vlessHandler).server.Port != 8443 {
		t.Fatal("non-default port discarded")
	}
}
