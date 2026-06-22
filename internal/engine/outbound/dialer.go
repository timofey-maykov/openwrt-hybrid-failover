package outbound

import (
	"context"
	"net"
	"strconv"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type singDialer struct {
	d *net.Dialer
}

func newSingDialer(bindIface string) N.Dialer {
	return &singDialer{d: outboundDialer(bindIface)}
}

func (s *singDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return s.d.DialContext(ctx, network, destination.String())
}

func (s *singDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, net.ErrClosed
}

func parseSocksaddrHostPort(host, port string) M.Socksaddr {
	p, _ := strconv.Atoi(port)
	if p <= 0 {
		p = 443
	}
	return M.ParseSocksaddrHostPort(host, uint16(p))
}

func parseDestAddr(address string) M.Socksaddr {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return M.ParseSocksaddr(address)
	}
	return parseSocksaddrHostPort(host, port)
}

type boundPacketConn struct {
	conn net.PacketConn
	dest *net.UDPAddr
}

func (c *boundPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	return c.conn.ReadFrom(p)
}

func (c *boundPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	_ = addr
	return c.conn.WriteTo(p, c.dest)
}

func (c *boundPacketConn) Close() error                       { return c.conn.Close() }
func (c *boundPacketConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *boundPacketConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *boundPacketConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *boundPacketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
