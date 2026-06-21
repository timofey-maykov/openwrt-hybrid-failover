package diag

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

const fakeipLookupTimeout = 2 * time.Second

func validateFakeIPResult(ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return fmt.Errorf("no fakeip response for %s", singbox.FAKEIPTestDomain)
	}
	if !strings.HasPrefix(ip, "198.18.") {
		return fmt.Errorf("unexpected fakeip address %q (want 198.18.x.x)", ip)
	}
	return nil
}

// lookupFakeIP resolves singbox.FAKEIPTestDomain via resolverHost (e.g. 127.0.0.42).
func lookupFakeIP(ctx context.Context, resolverHost, qname string) (string, error) {
	resolverHost = strings.TrimSpace(resolverHost)
	if resolverHost == "" {
		return "", fmt.Errorf("empty resolver address")
	}
	addr := resolverHost
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "53")
	}

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = fakeipLookupTimeout
			return d.DialContext(ctx, "udp", addr)
		},
	}

	ips, err := r.LookupHost(ctx, qname)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if strings.HasPrefix(ip, "198.18.") {
			return ip, nil
		}
	}
	if len(ips) > 0 {
		return ips[0], nil
	}
	return "", nil
}
