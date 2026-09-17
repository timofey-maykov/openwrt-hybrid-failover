package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/sagernet/sing-quic/hysteria"
	"github.com/sagernet/sing-quic/hysteria2"
	N "github.com/sagernet/sing/common/network"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

type hysteria2Client struct {
	tag    string
	client *hysteria2.Client
}

func newHysteria2Handler(ctx context.Context, p plan.OutboundPlan) (Handler, error) {
	ob, err := uri.ParseProxy(p.ProxyURI, p.Tag, false)
	if err != nil {
		return nil, err
	}
	server, _ := ob.Fields["server"].(string)
	port := numericField(ob.Fields["server_port"])
	password, _ := ob.Fields["password"].(string)
	if server == "" || port < 1 || port > 65535 || password == "" {
		return nil, fmt.Errorf("hysteria2: invalid server, port, or password")
	}
	tlsCfg, err := buildTLSConfig(ob.Fields, server)
	if err != nil {
		return nil, err
	}
	if tlsCfg == nil {
		tlsCfg, _ = buildTLSConfig(map[string]any{"tls": map[string]any{"enabled": true, "server_name": server}}, server)
	}
	var salamander string
	if obfs, ok := ob.Fields["obfs"].(map[string]any); ok {
		if t, _ := obfs["type"].(string); t == "salamander" {
			salamander, _ = obfs["password"].(string)
		}
	}
	sendBPS := uint64(0)
	recvBPS := uint64(0)
	if v := numericField(ob.Fields["up_mbps"]); v > 0 {
		sendBPS = uint64(v) * hysteria.MbpsToBps
	}
	if v := numericField(ob.Fields["down_mbps"]); v > 0 {
		recvBPS = uint64(v) * hysteria.MbpsToBps
	}
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := hysteria2.NewClient(hysteria2.ClientOptions{
		Context:            ctx,
		Dialer:             newSingDialer(p.BindIface),
		Logger:             newNopLogger(),
		ServerAddress:      parseSocksaddrHostPort(server, strconv.Itoa(port)),
		Password:           password,
		SalamanderPassword: salamander,
		SendBPS:            sendBPS,
		ReceiveBPS:         recvBPS,
		TLSConfig:          tlsCfg,
		UDPDisabled:        false,
	})
	if err != nil {
		return nil, err
	}
	return &hysteria2Client{tag: p.Tag, client: client}, nil
}

func (h *hysteria2Client) Tag() string { return h.tag }

func (h *hysteria2Client) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	return h.client.DialConn(ctx, parseDestAddr(address))
}

func (h *hysteria2Client) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	pc, err := h.client.ListenPacket(ctx)
	if err != nil {
		return nil, err
	}
	dest := parseDestAddr(address)
	return &boundPacketConn{conn: pc, dest: dest}, nil
}

func (h *hysteria2Client) Close() error {
	h.client.CloseWithError(nil)
	return nil
}

var _ N.Dialer = (*singDialer)(nil)
