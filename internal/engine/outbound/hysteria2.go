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

func newHysteria2Handler(p plan.OutboundPlan) (Handler, error) {
	ob, err := uri.ParseProxy(p.ProxyURI, p.Tag, false)
	if err != nil {
		return nil, err
	}
	server, _ := ob.Fields["server"].(string)
	portF, _ := ob.Fields["server_port"].(float64)
	password, _ := ob.Fields["password"].(string)
	if server == "" || password == "" {
		return nil, fmt.Errorf("hysteria2: missing server or password")
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
	if v, ok := ob.Fields["up_mbps"].(float64); ok && v > 0 {
		sendBPS = uint64(v) * hysteria.MbpsToBps
	}
	if v, ok := ob.Fields["down_mbps"].(float64); ok && v > 0 {
		recvBPS = uint64(v) * hysteria.MbpsToBps
	}
	client, err := hysteria2.NewClient(hysteria2.ClientOptions{
		Context:            context.Background(),
		Dialer:             newSingDialer(""),
		Logger:             newNopLogger(),
		ServerAddress:      parseSocksaddrHostPort(server, strconv.Itoa(int(portF))),
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
	return &boundPacketConn{conn: pc, dest: dest.UDPAddr()}, nil
}

func (h *hysteria2Client) Close() error {
	h.client.CloseWithError(nil)
	return nil
}

var _ N.Dialer = (*singDialer)(nil)
