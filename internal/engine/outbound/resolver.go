package outbound

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

// realDNSResolver resolves names via public/bootstrap DNS.
// Never use the engine FakeIP listener (127.0.0.42): bind/VPN outbounds would
// dial 198.18.x.x through the tunnel and hang.
func realDNSResolver() *net.Resolver {
	servers := []string{
		net.JoinHostPort(singbox.DefaultBootstrapDNS, "53"),
		net.JoinHostPort(singbox.DefaultDNSServer, "53"),
		"8.8.8.8:53",
	}
	var next atomic.Uint32
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 4 * time.Second, Control: engineSocketControl("")}
			// A UDP dial only fails on local errors (bad address/routing), never
			// because the remote server is down, so "first that dials" always
			// picked the same server. Round-robin across attempts instead, so
			// the net resolver's own per-query retries actually reach the
			// other configured servers when one is unreachable.
			idx := int(next.Add(1)-1) % len(servers)
			target := servers[idx]
			var lastErr error
			// socket() can transiently fail with ENFILE/EMFILE under a burst of
			// concurrent tproxy'd connections plus health-check dials on
			// constrained hardware. The burst has been observed to outlast a
			// sub-second retry budget, so retry with growing backoff (up to ~3s
			// total) instead of surfacing it as a real DNS/probe failure.
			backoff := 20 * time.Millisecond
			for attempt := 0; attempt < 10; attempt++ {
				conn, err := d.DialContext(ctx, "udp4", target)
				if err == nil {
					return conn, nil
				}
				lastErr = err
				if !isTransientDialError(err) {
					break
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				if backoff < 500*time.Millisecond {
					backoff *= 2
				}
			}
			return nil, lastErr
		},
	}
}

func isTransientDialError(err error) bool {
	return errors.Is(err, syscall.ENFILE) || errors.Is(err, syscall.EMFILE)
}

func outboundDialer(bindIface string) *net.Dialer {
	return &net.Dialer{
		Timeout:  30 * time.Second,
		Resolver: realDNSResolver(),
		Control:  engineSocketControl(bindIface),
	}
}
