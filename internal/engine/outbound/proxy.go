package outbound

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	shadowsocks "github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	M "github.com/sagernet/sing/common/metadata"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

type proxyHandler struct {
	tag                         string
	kind                        plan.OutboundKind
	bindIface                   string
	server                      string
	username, password, version string
	tls                         aTLS.Config
	method                      shadowsocks.Method
}

func numericField(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func newProxyHandler(p plan.OutboundPlan) (Handler, error) {
	ob, err := uri.ParseProxy(p.ProxyURI, p.Tag, false)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(p.ProxyURI)
	if err != nil {
		return nil, err
	}
	if ob.Fields["transport"] != nil || u.Query().Get("plugin") != "" {
		return nil, fmt.Errorf("%s: transport/plugin is not supported by the native engine", p.Kind)
	}
	server, _ := ob.Fields["server"].(string)
	port := numericField(ob.Fields["server_port"])
	if server == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%s: invalid server address", p.Kind)
	}
	h := &proxyHandler{tag: p.Tag, kind: p.Kind, bindIface: p.BindIface, server: net.JoinHostPort(server, strconv.Itoa(port))}
	h.password, _ = ob.Fields["password"].(string)
	h.username, _ = ob.Fields["username"].(string)
	h.version, _ = ob.Fields["version"].(string)
	switch p.Kind {
	case plan.OutboundShadowsocks:
		method, _ := ob.Fields["method"].(string)
		h.method, err = shadowimpl.FetchMethod(method, h.password, time.Now)
		if err != nil {
			return nil, err
		}
	case plan.OutboundTrojan:
		if h.password == "" {
			return nil, fmt.Errorf("trojan: password is required")
		}
		// Trojan requires TLS unless a supported explicit TLS configuration overrides it.
		fields, _ := ob.Fields["tls"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
			ob.Fields["tls"] = fields
		}
		fields["enabled"] = true
		if fields["server_name"] == nil || fields["server_name"] == "" {
			name := u.Query().Get("sni")
			if name == "" {
				name = server
			}
			fields["server_name"] = name
		}
		for _, key := range []string{"insecure", "allowInsecure", "allow_insecure"} {
			if v := u.Query().Get(key); v == "1" || strings.EqualFold(v, "true") {
				fields["insecure"] = true
			}
		}
		h.tls, err = buildTLSConfig(ob.Fields, server)
		if err != nil {
			return nil, err
		}
	case plan.OutboundSocks:
		if u.User != nil {
			h.username = u.User.Username()
			h.password, _ = u.User.Password()
		}
	default:
		return nil, fmt.Errorf("unsupported proxy kind %q", p.Kind)
	}
	return h, nil
}

func (p *proxyHandler) Tag() string  { return p.tag }
func (p *proxyHandler) Close() error { return nil }

// Bound both reads and writes of protocol handshakes, even for callers with no
// deadline. Stop the cancellation callback before giving the connection away.
func handshake(ctx context.Context, c net.Conn, fn func() error) error {
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = c.SetDeadline(deadline)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = c.Close(); close(done) })
	err := fn()
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = c.Close()
		return err
	}
	return c.SetDeadline(time.Time{})
}

func (p *proxyHandler) dialServer(ctx context.Context) (net.Conn, error) {
	return outboundDialer(p.bindIface).DialContext(ctx, "tcp", p.server)
}

func (p *proxyHandler) socksHandshake(ctx context.Context, c net.Conn, dest M.Socksaddr, command byte) (socks5.Response, error) {
	var result socks5.Response
	if p.version == "4" && dest.IsFqdn() {
		ips, err := realDNSResolver().LookupIP(ctx, "ip4", dest.Fqdn)
		if err != nil || len(ips) == 0 {
			_ = c.Close()
			return result, fmt.Errorf("socks4: resolve destination: %v", err)
		}
		dest = M.SocksaddrFromNet(&net.TCPAddr{IP: ips[0], Port: int(dest.Port)})
	}
	err := handshake(ctx, c, func() error {
		switch p.version {
		case "4", "4a":
			_, err := socks.ClientHandshake4(c, command, dest, p.username)
			return err
		case "5":
			var err error
			result, err = socks.ClientHandshake5(c, command, dest, p.username, p.password)
			return err
		default:
			return fmt.Errorf("unsupported SOCKS version %q", p.version)
		}
	})
	return result, err
}

func (p *proxyHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	dest := parseDestAddr(address)
	conn, err := p.dialServer(ctx)
	if err != nil {
		return nil, err
	}
	switch p.kind {
	case plan.OutboundSocks:
		if _, err = p.socksHandshake(ctx, conn, dest, socks5.CommandConnect); err != nil {
			return nil, err
		}
		return conn, nil
	case plan.OutboundShadowsocks:
		var encrypted net.Conn
		err = handshake(ctx, conn, func() error { var e error; encrypted, e = p.method.DialConn(conn, dest); return e })
		if err != nil {
			return nil, err
		}
		return encrypted, nil
	case plan.OutboundTrojan:
		var stream net.Conn
		err = handshake(ctx, conn, func() error {
			var e error
			stream, e = dialTLS(ctx, conn, p.tls)
			if e != nil {
				return e
			}
			return writeTrojanRequest(stream, p.password, 1, dest)
		})
		if err != nil {
			return nil, err
		}
		return stream, nil
	}
	_ = conn.Close()
	return nil, fmt.Errorf("unsupported proxy kind %q", p.kind)
}

func writeTrojanRequest(conn net.Conn, password string, command byte, dest M.Socksaddr) error {
	hash := sha256.Sum224([]byte(password))
	var header bytes.Buffer
	header.WriteString(hex.EncodeToString(hash[:]) + "\r\n")
	header.WriteByte(command)
	if err := M.SocksaddrSerializer.WriteAddrPort(&header, dest); err != nil {
		return err
	}
	header.WriteString("\r\n")
	_, err := conn.Write(header.Bytes())
	return err
}

func (p *proxyHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	dest := parseDestAddr(address)
	switch p.kind {
	case plan.OutboundShadowsocks:
		conn, err := outboundDialer(p.bindIface).DialContext(ctx, "udp", p.server)
		if err != nil {
			return nil, err
		}
		return &boundPacketConn{conn: p.method.DialPacketConn(conn), dest: dest}, nil
	case plan.OutboundSocks:
		if p.version != "5" {
			return nil, fmt.Errorf("SOCKS%s does not support UDP", p.version)
		}
		tcp, err := p.dialServer(ctx)
		if err != nil {
			return nil, err
		}
		response, err := p.socksHandshake(ctx, tcp, dest, socks5.CommandUDPAssociate)
		if err != nil {
			return nil, err
		}
		relay := response.Bind
		if !relay.IsFqdn() && (!relay.Addr.IsValid() || relay.Addr.IsUnspecified()) {
			relay.Addr = M.SocksaddrFromNet(tcp.RemoteAddr()).Addr
		}
		udp, err := outboundDialer(p.bindIface).DialContext(ctx, "udp", relay.String())
		if err != nil {
			_ = tcp.Close()
			return nil, err
		}
		return &boundPacketConn{conn: socks.NewAssociatePacketConn(udp, dest, tcp), dest: dest}, nil
	case plan.OutboundTrojan:
		conn, err := p.dialServer(ctx)
		if err != nil {
			return nil, err
		}
		var stream net.Conn
		err = handshake(ctx, conn, func() error {
			var e error
			stream, e = dialTLS(ctx, conn, p.tls)
			if e != nil {
				return e
			}
			return writeTrojanRequest(stream, p.password, 3, dest)
		})
		if err != nil {
			return nil, err
		}
		return &boundPacketConn{conn: &trojanPacketConn{Conn: stream}, dest: dest}, nil
	}
	return nil, fmt.Errorf("unsupported proxy kind %q", p.kind)
}
