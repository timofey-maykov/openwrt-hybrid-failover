package dns

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

// answer serves one A record for every query on the given listener/conn.
func answer(t *testing.T, ip string, udp net.PacketConn, tcp net.Listener) {
	t.Helper()
	h := mdns.HandlerFunc(func(w mdns.ResponseWriter, r *mdns.Msg) {
		m := new(mdns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &mdns.A{Hdr: mdns.RR_Header{Name: r.Question[0].Name, Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 60}, A: net.ParseIP(ip).To4()})
		_ = w.WriteMsg(m)
	})
	if udp != nil {
		srv := &mdns.Server{PacketConn: udp, Handler: h}
		go func() { _ = srv.ActivateAndServe() }()
		t.Cleanup(func() { _ = srv.Shutdown() })
	}
	if tcp != nil {
		srv := &mdns.Server{Listener: tcp, Handler: h}
		go func() { _ = srv.ActivateAndServe() }()
		t.Cleanup(func() { _ = srv.Shutdown() })
	}
}

type tcpDialer struct {
	addr string
	err  error
}

func (d tcpDialer) DialUpstream(ctx context.Context, _ string) (net.Conn, error) {
	if d.err != nil {
		return nil, d.err
	}
	return (&net.Dialer{}).DialContext(ctx, "tcp", d.addr)
}

func udpUpstream(t *testing.T, ip string) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	answer(t, ip, pc, nil)
	return pc.LocalAddr().String()
}

func tcpUpstream(t *testing.T, ip string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	answer(t, ip, nil, ln)
	return ln.Addr().String()
}

func deadUDP(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	_ = pc.Close() // nobody answers there; a query times out
	return addr
}

func lookup(t *testing.T, mode, bootstrap string, d UpstreamDialer) (net.IP, error) {
	t.Helper()
	s := NewServer(Plan{Bootstrap: bootstrap, UpstreamVia: mode}, nil)
	if d != nil {
		s.SetUpstreamDialer(d)
	}
	return s.upstreamLookup("example.org")
}

func TestUpstreamWANOnly(t *testing.T) {
	ip, err := lookup(t, UpstreamWAN, udpUpstream(t, "1.2.3.4"), tcpDialer{addr: tcpUpstream(t, "5.6.7.8")})
	if err != nil || ip.String() != "1.2.3.4" {
		t.Fatalf("got %v %v", ip, err)
	}
}

func TestUpstreamTunnelOnly(t *testing.T) {
	ip, err := lookup(t, UpstreamTunnel, udpUpstream(t, "1.2.3.4"), tcpDialer{addr: tcpUpstream(t, "5.6.7.8")})
	if err != nil || ip.String() != "5.6.7.8" {
		t.Fatalf("got %v %v", ip, err)
	}
	if _, err := lookup(t, UpstreamTunnel, udpUpstream(t, "1.2.3.4"), nil); err == nil {
		t.Fatal("tunnel mode without a dialer must fail, not fall back silently")
	}
}

func TestUpstreamAutoSurvivesADeadWAN(t *testing.T) {
	start := time.Now()
	ip, err := lookup(t, UpstreamAuto, deadUDP(t), tcpDialer{addr: tcpUpstream(t, "5.6.7.8")})
	if err != nil || ip.String() != "5.6.7.8" {
		t.Fatalf("got %v %v", ip, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("waited for the dead WAN leg: %v", time.Since(start))
	}
}

func TestUpstreamAutoSurvivesADeadTunnel(t *testing.T) {
	ip, err := lookup(t, UpstreamAuto, udpUpstream(t, "1.2.3.4"), tcpDialer{err: errors.New("no channel is up")})
	if err != nil || ip.String() != "1.2.3.4" {
		t.Fatalf("got %v %v", ip, err)
	}
}

func TestUpstreamAutoReportsBothFailures(t *testing.T) {
	old := upstreamTimeoutForTest(300 * time.Millisecond)
	defer old()
	_, err := lookup(t, UpstreamAuto, deadUDP(t), tcpDialer{err: errors.New("no channel is up")})
	if err == nil || !strings.Contains(err.Error(), "wan:") || !strings.Contains(err.Error(), "tunnel:") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpstreamWithoutDialerIsPlainWAN(t *testing.T) {
	ip, err := lookup(t, UpstreamAuto, udpUpstream(t, "1.2.3.4"), nil)
	if err != nil || ip.String() != "1.2.3.4" {
		t.Fatalf("got %v %v", ip, err)
	}
}

func TestNormalizeUpstreamVia(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "AUTO": "auto", "wan": "wan", "direct": "wan", "tunnel": "tunnel", "proxy": "tunnel", "nonsense": "auto"} {
		if got := NormalizeUpstreamVia(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
