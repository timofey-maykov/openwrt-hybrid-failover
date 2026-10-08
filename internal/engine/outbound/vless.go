package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/sagernet/sing-vmess/vless"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

type vlessHandler struct {
	tag    string
	client *vless.Client
	server M.Socksaddr
	tls    aTLS.Config
	dialer N.Dialer

	transport vlessTransport
	// serverName is the name used for the TLS handshake and as the HTTP/2
	// authority of the grpc transport.
	serverName string
	// initErr is set for a link this engine cannot carry (an unknown
	// transport, Vision over ws). It is returned when the channel is used, not
	// when it is created: an error at creation stops the whole engine.
	initErr error
}

// vlessTransport is what the link says about carrying the stream: plain TCP,
// WebSocket or gRPC.
type vlessTransport struct {
	kind    string // "", "ws" or "grpc"
	host    string // Host header of ws
	path    string
	headers map[string]string
	grpc    grpcOptions
}

func vlessTransportFrom(fields map[string]any, flow, tlsName string) (vlessTransport, error) {
	var t vlessTransport
	raw, _ := fields["transport"].(map[string]any)
	kind, _ := raw["type"].(string)
	switch kind {
	case "", "tcp":
		return t, nil
	case "ws":
		t.kind = "ws"
		t.path, _ = raw["path"].(string)
		t.headers = map[string]string{}
		switch h := raw["headers"].(type) {
		case map[string]string:
			for k, v := range h {
				t.headers[k] = v
			}
		case map[string]any:
			for k, v := range h {
				if sv, ok := v.(string); ok {
					t.headers[k] = sv
				}
			}
		}
		t.host = t.headers["Host"]
		if t.host == "" {
			t.host = tlsName
		}
		delete(t.headers, "Host")
	case "grpc":
		t.kind = "grpc"
		t.grpc.service, _ = raw["service_name"].(string)
		t.grpc.authority, _ = raw["authority"].(string)
		if v, _ := raw["idle_timeout"].(string); v != "" {
			t.grpc.idleTimeout, _ = time.ParseDuration(v)
		}
		if v, _ := raw["ping_timeout"].(string); v != "" {
			t.grpc.pingTimeout, _ = time.ParseDuration(v)
		}
	default:
		return t, fmt.Errorf("vless: transport %q is not supported by the native engine (tcp, ws and grpc are)", kind)
	}
	if flow == vless.FlowVision {
		return t, fmt.Errorf("vless: flow xtls-rprx-vision works on plain TCP only, not over %s", t.kind)
	}
	return t, nil
}

func newVLESSHandler(p plan.OutboundPlan) (Handler, error) {
	ob, err := uri.ParseProxy(p.ProxyURI, p.Tag, false)
	if err != nil {
		return nil, err
	}
	server, _ := ob.Fields["server"].(string)
	port := numericField(ob.Fields["server_port"])
	if server == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("vless: invalid server or port")
	}
	uuid, _ := ob.Fields["uuid"].(string)
	flow, _ := ob.Fields["flow"].(string)
	client, err := vless.NewClient(uuid, flow, newNopLogger())
	if err != nil {
		return nil, err
	}
	tlsCfg, err := buildTLSConfig(ob.Fields, server)
	if err != nil {
		return nil, err
	}
	name := server
	if tlsCfg != nil && tlsCfg.ServerName() != "" {
		name = tlsCfg.ServerName()
	}
	transport, initErr := vlessTransportFrom(ob.Fields, flow, name)
	if tlsCfg != nil {
		// The ALPN has to match the transport: a server that picks h2 for a
		// WebSocket upgrade, or http/1.1 for gRPC, answers nothing useful.
		userALPN := false
		if raw, ok := ob.Fields["tls"].(map[string]any); ok {
			userALPN = len(alpnFromField(raw["alpn"])) > 0
		}
		switch transport.kind {
		case "ws":
			if !userALPN {
				tlsCfg.SetNextProtos([]string{"http/1.1"})
			}
		case "grpc":
			tlsCfg.SetNextProtos([]string{"h2"})
		}
	}
	return &vlessHandler{
		tag:        p.Tag,
		client:     client,
		server:     parseSocksaddrHostPort(server, strconv.Itoa(port)),
		tls:        tlsCfg,
		dialer:     newSingDialer(p.BindIface),
		transport:  transport,
		serverName: name,
		initErr:    initErr,
	}, nil
}

func (h *vlessHandler) Tag() string { return h.tag }

// dialStream connects to the server and puts the transport the link asks for
// on top of TLS. The VLESS header goes over whatever this returns.
func (h *vlessHandler) dialStream(ctx context.Context) (net.Conn, error) {
	if h.initErr != nil {
		return nil, h.initErr
	}
	conn, err := h.dialer.DialContext(ctx, "tcp", h.server)
	if err != nil {
		return nil, err
	}
	tlsConn, err := dialTLS(ctx, conn, h.tls)
	if err != nil {
		return nil, err
	}
	switch h.transport.kind {
	case "ws":
		ws, err := newWebSocketClient(ctx, tlsConn, h.transport.host, h.transport.path, h.transport.headers)
		if err != nil {
			_ = tlsConn.Close()
			return nil, err
		}
		return ws, nil
	case "grpc":
		g, err := newGRPCClient(tlsConn, h.serverName, h.transport.grpc)
		if err != nil {
			_ = tlsConn.Close()
			return nil, err
		}
		return g, nil
	}
	return tlsConn, nil
}

func (h *vlessHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	stream, err := h.dialStream(ctx)
	if err != nil {
		return nil, err
	}
	dest := parseDestAddr(address)
	early, err := h.client.DialEarlyConn(stream, dest)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	if _, err := early.Write(nil); err != nil {
		_ = early.Close()
		return nil, err
	}
	return early, nil
}

// DialUDP carries UDP over VLESS as XUDP, the same way xray and sing-box do.
// XUDP also is the only form servers accept together with flow
// xtls-rprx-vision. Each call opens its own connection, like the other
// handlers do for a UDP flow.
func (h *vlessHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	stream, err := h.dialStream(ctx)
	if err != nil {
		return nil, err
	}
	dest := parseDestAddr(address)
	pc, err := h.client.DialEarlyXUDPPacketConn(stream, dest)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	return &boundPacketConn{conn: pc, dest: dest}, nil
}

func (h *vlessHandler) Close() error { return nil }
