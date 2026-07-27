package outbound

import (
	"context"
	"net"
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
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 4 * time.Second}
			var last error
			for _, dns := range servers {
				c, err := d.DialContext(ctx, "udp4", dns)
				if err == nil {
					return c, nil
				}
				last = err
			}
			if last != nil {
				return nil, last
			}
			return d.DialContext(ctx, "udp4", servers[0])
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
