package outbound

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

type proxyHandler struct {
	tag  string
	kind plan.OutboundKind
	uri  string
}

func newProxyHandler(p plan.OutboundPlan) (Handler, error) {
	if p.ProxyURI == "" {
		return nil, fmt.Errorf("empty proxy uri")
	}
	return &proxyHandler{tag: p.Tag, kind: p.Kind, uri: p.ProxyURI}, nil
}

func (p *proxyHandler) Tag() string { return p.tag }

func (p *proxyHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(p.uri)
	if err != nil {
		return nil, err
	}
	serverHost := u.Hostname()
	serverPort := u.Port()
	if serverPort == "" {
		serverPort = "443"
	}
	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(serverHost, serverPort))
	if err != nil {
		return nil, err
	}
	if needsTLS(p.kind, u) {
		sni := u.Query().Get("sni")
		if sni == "" {
			sni = serverHost
		}
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         sni,
			InsecureSkipVerify: true,
			NextProtos:         []string{"h2", "http/1.1"},
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
	_ = host
	_ = port
	return conn, nil
}

func (p *proxyHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, fmt.Errorf("udp not supported for %s yet", p.kind)
}

func (p *proxyHandler) Close() error { return nil }

func needsTLS(kind plan.OutboundKind, u *url.URL) bool {
	switch kind {
	case plan.OutboundTrojan:
		return true
	default:
		return strings.Contains(strings.ToLower(u.Scheme), "https") || u.Query().Get("security") == "tls"
	}
}
