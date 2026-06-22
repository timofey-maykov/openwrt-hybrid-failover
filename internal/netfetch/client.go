package netfetch

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultDNSAddr  = "127.0.0.1:53"
	SingboxDNSAddr  = "127.0.0.42:53"
	defaultTimeout  = 60 * time.Second
	dnsDialTimeout  = 5 * time.Second
	connDialTimeout = 30 * time.Second
)

// HTTPClient builds an HTTP client with explicit DNS over udp4.
// proxyURL, when set, routes HTTP(S) through a local proxy (for example sing-box mixed inbound).
func HTTPClient(dnsAddr, proxyURL string, timeout time.Duration) *http.Client {
	if dnsAddr == "" {
		dnsAddr = DefaultDNSAddr
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: dnsDialTimeout}
			return d.DialContext(ctx, "udp4", dnsAddr)
		},
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   connDialTimeout,
				KeepAlive: 30 * time.Second,
				Resolver:  resolver,
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
