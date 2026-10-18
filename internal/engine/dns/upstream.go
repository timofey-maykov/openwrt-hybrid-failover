package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	mdns "github.com/miekg/dns"
)

// How queries that get no FakeIP answer reach the upstream resolver.
//
// Plain UDP/53 from the WAN is the fast path, but some providers drop or
// hijack it now and then; when that happens every site outside the lists
// stops resolving while the listed ones keep working through the tunnel.
// The tunnel path sends the same query over TCP through the active channel.
const (
	UpstreamAuto   = "auto"   // both at once, first good answer wins
	UpstreamWAN    = "wan"    // plain UDP only
	UpstreamTunnel = "tunnel" // through the active channel only
)

var upstreamTimeout = 4 * time.Second

// UpstreamDialer opens a TCP connection to addr through the routing engine.
type UpstreamDialer interface {
	DialUpstream(ctx context.Context, addr string) (net.Conn, error)
}

// SetUpstreamDialer enables the tunnel path.
func (s *Server) SetUpstreamDialer(d UpstreamDialer) {
	s.mu.Lock()
	s.upstream = d
	s.mu.Unlock()
}

// NormalizeUpstreamVia maps the UCI value to a mode, auto for anything odd.
func NormalizeUpstreamVia(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case UpstreamWAN, "direct":
		return UpstreamWAN
	case UpstreamTunnel, "proxy", "vpn":
		return UpstreamTunnel
	default:
		return UpstreamAuto
	}
}

// upstreamAddr is host:53, or the configured value when it already has a port.
func (s *Server) upstreamAddr() string {
	if _, _, err := net.SplitHostPort(s.plan.Bootstrap); err == nil {
		return s.plan.Bootstrap
	}
	return net.JoinHostPort(s.plan.Bootstrap, "53")
}

func (s *Server) upstreamLookup(name string) (net.IP, error) {
	s.mu.Lock()
	dialer := s.upstream
	s.mu.Unlock()
	mode := NormalizeUpstreamVia(s.plan.UpstreamVia)
	if dialer == nil && mode == UpstreamTunnel {
		return nil, errors.New("dns upstream via tunnel: no tunnel dialer")
	}
	if dialer == nil {
		mode = UpstreamWAN
	}
	ctx, cancel := context.WithTimeout(context.Background(), upstreamTimeout)
	defer cancel()
	addr := s.upstreamAddr()
	msg := mdnsMsg(name)

	switch mode {
	case UpstreamWAN:
		return queryWAN(ctx, msg, addr)
	case UpstreamTunnel:
		return queryTunnel(ctx, dialer, msg, addr)
	}

	type result struct {
		ip  net.IP
		err error
		via string
	}
	results := make(chan result, 2)
	go func() { ip, err := queryWAN(ctx, msg, addr); results <- result{ip, err, "wan"} }()
	go func() { ip, err := queryTunnel(ctx, dialer, msg, addr); results <- result{ip, err, "tunnel"} }()
	var errs []string
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			return r.ip, nil
		}
		errs = append(errs, r.via+": "+r.err.Error())
	}
	return nil, errors.New(strings.Join(errs, "; "))
}

func queryWAN(ctx context.Context, msg *mdns.Msg, addr string) (net.IP, error) {
	c := mdns.Client{Timeout: upstreamTimeout}
	resp, _, err := c.ExchangeContext(ctx, msg, addr)
	if err != nil {
		return nil, err
	}
	return firstA(resp)
}

func queryTunnel(ctx context.Context, d UpstreamDialer, msg *mdns.Msg, addr string) (net.IP, error) {
	conn, err := d.DialUpstream(ctx, addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c := mdns.Client{Net: "tcp", Timeout: upstreamTimeout}
	resp, _, err := c.ExchangeWithConnContext(ctx, msg, &mdns.Conn{Conn: conn})
	if err != nil {
		return nil, err
	}
	return firstA(resp)
}

func firstA(resp *mdns.Msg) (net.IP, error) {
	for _, ans := range resp.Answer {
		if a, ok := ans.(*mdns.A); ok {
			return a.A, nil
		}
	}
	return nil, fmt.Errorf("no A record")
}

// upstreamTimeoutForTest shortens the upstream timeout and returns a restore function.
func upstreamTimeoutForTest(d time.Duration) func() {
	prev := upstreamTimeout
	upstreamTimeout = d
	return func() { upstreamTimeout = prev }
}
