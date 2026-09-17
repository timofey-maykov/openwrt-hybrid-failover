package outbound

import (
	"context"
	"fmt"
	"net"
	"strconv"

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
	return &vlessHandler{
		tag:    p.Tag,
		client: client,
		server: parseSocksaddrHostPort(server, strconv.Itoa(port)),
		tls:    tlsCfg,
		dialer: newSingDialer(p.BindIface),
	}, nil
}

func (h *vlessHandler) Tag() string { return h.tag }

func (h *vlessHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := h.dialer.DialContext(ctx, "tcp", h.server)
	if err != nil {
		return nil, err
	}
	tlsConn, err := dialTLS(ctx, conn, h.tls)
	if err != nil {
		return nil, err
	}
	dest := parseDestAddr(address)
	early, err := h.client.DialEarlyConn(tlsConn, dest)
	if err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	if _, err := early.Write(nil); err != nil {
		_ = early.Close()
		return nil, err
	}
	return early, nil
}

func (h *vlessHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, fmt.Errorf("vless udp not implemented")
}

func (h *vlessHandler) Close() error { return nil }
