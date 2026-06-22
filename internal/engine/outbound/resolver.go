package outbound

import (
	"context"
	"net"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
)

func engineDNSResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp4", netfetch.SingboxDNSAddr)
		},
	}
}

func outboundDialer(bindIface string) *net.Dialer {
	d := &net.Dialer{
		Timeout:  30 * time.Second,
		Resolver: engineDNSResolver(),
	}
	if bindIface != "" {
		d.Control = bindToDevice(bindIface)
	}
	return d
}
