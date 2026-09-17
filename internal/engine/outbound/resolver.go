package outbound

import (
	"context"
	"net"
	"sync/atomic"
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
			d := net.Dialer{Timeout: 4 * time.Second}
			// A UDP dial only fails on local errors (bad address/routing), never
			// because the remote server is down, so "first that dials" always
			// picked the same server. Round-robin across attempts instead, so
			// the net resolver's own per-query retries actually reach the
			// other configured servers when one is unreachable.
			idx := int(next.Add(1)-1) % len(servers)
			return d.DialContext(ctx, "udp4", servers[idx])
		},
	}
}

func outboundDialer(bindIface string) *net.Dialer {
	d := &net.Dialer{
		Timeout:  30 * time.Second,
		Resolver: realDNSResolver(),
	}
	if bindIface != "" {
		d.Control = bindToDevice(bindIface)
	}
	return d
}
