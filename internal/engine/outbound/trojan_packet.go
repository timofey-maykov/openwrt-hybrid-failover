package outbound

import (
	"bytes"
	"encoding/binary"
	"fmt"
	M "github.com/sagernet/sing/common/metadata"
	"io"
	"net"
	"sync"
)

// Trojan carries UDP packets as length-delimited records inside its TLS stream.
type trojanPacketConn struct {
	net.Conn
	readMu, writeMu sync.Mutex
}

func (c *trojanPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if len(p) > 65535 {
		return 0, fmt.Errorf("trojan: UDP packet too large")
	}
	var frame bytes.Buffer
	if err := M.SocksaddrSerializer.WriteAddrPort(&frame, M.SocksaddrFromNet(addr)); err != nil {
		return 0, err
	}
	_ = binary.Write(&frame, binary.BigEndian, uint16(len(p)))
	frame.WriteString("\r\n")
	frame.Write(p)
	if _, err := c.Conn.Write(frame.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (c *trojanPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	addr, err := M.SocksaddrSerializer.ReadAddrPort(c.Conn)
	if err != nil {
		return 0, nil, err
	}
	var header [4]byte
	if _, err = io.ReadFull(c.Conn, header[:]); err != nil {
		return 0, nil, err
	}
	if header[2] != '\r' || header[3] != '\n' {
		return 0, nil, fmt.Errorf("trojan: invalid UDP delimiter")
	}
	size := int(binary.BigEndian.Uint16(header[:2]))
	if size > len(p) {
		n, err := io.ReadFull(c.Conn, p)
		if err != nil {
			return n, addr, err
		}
		_, err = io.CopyN(io.Discard, c.Conn, int64(size-len(p)))
		if err != nil {
			return n, addr, err
		}
		return n, addr, io.ErrShortBuffer
	}
	n, err := io.ReadFull(c.Conn, p[:size])
	return n, addr, err
}
