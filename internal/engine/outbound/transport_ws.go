package outbound

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // fixed by RFC 6455, not a security use
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A small WebSocket client (RFC 6455) for the VLESS ws transport. It carries
// the byte stream in binary messages, which is what xray and sing-box servers
// expect, and answers pings. It is deliberately not a general WebSocket
// library: no extensions, no text messages, no subprotocols.

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsMaxFrame bounds the payload of one outgoing frame.
const wsMaxFrame = 1 << 15

const wsDefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// newWebSocketClient upgrades conn (already TLS or plain TCP to the server).
// host is the Host header, path may carry a query.
func newWebSocketClient(ctx context.Context, conn net.Conn, host, path string, headers map[string]string) (net.Conn, error) {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
		defer conn.SetDeadline(time.Time{})
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	keyRaw := make([]byte, 16)
	if _, err := rand.Read(keyRaw); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyRaw)

	var req strings.Builder
	fmt.Fprintf(&req, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&req, "Host: %s\r\n", host)
	req.WriteString("Upgrade: websocket\r\nConnection: Upgrade\r\n")
	fmt.Fprintf(&req, "Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n", key)
	userAgentSet := false
	for k, v := range headers {
		// the handshake owns these; a header from a link must not replace them
		switch http.CanonicalHeaderKey(k) {
		case "Host", "Upgrade", "Connection", "Sec-Websocket-Key", "Sec-Websocket-Version":
			continue
		case "User-Agent":
			userAgentSet = true
		}
		if strings.ContainsAny(k, "\r\n:") || strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("websocket: bad header in the link")
		}
		fmt.Fprintf(&req, "%s: %s\r\n", k, v)
	}
	if !userAgentSet {
		fmt.Fprintf(&req, "User-Agent: %s\r\n", wsDefaultUserAgent)
	}
	req.WriteString("\r\n")
	if _, err := io.WriteString(conn, req.String()); err != nil {
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		return nil, fmt.Errorf("websocket handshake: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("websocket handshake: server answered %s", resp.Status)
	}
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec
	if resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		return nil, errors.New("websocket handshake: wrong Sec-WebSocket-Accept")
	}
	return newWSConn(conn, br, true), nil
}

// wsConn is one side of an established WebSocket as a net.Conn.
type wsConn struct {
	conn   net.Conn
	br     *bufio.Reader
	client bool // clients mask what they send, servers do not

	rmu       sync.Mutex
	remaining int64 // payload bytes of the current data frame still to read
	masked    bool
	mask      [4]byte
	maskPos   int

	wmu    sync.Mutex
	closed bool
}

func newWSConn(conn net.Conn, br *bufio.Reader, client bool) *wsConn {
	return &wsConn{conn: conn, br: br, client: client}
}

func (c *wsConn) Read(p []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	for c.remaining == 0 {
		if err := c.nextFrame(); err != nil {
			return 0, err
		}
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.br.Read(p)
	if c.masked {
		for i := 0; i < n; i++ {
			p[i] ^= c.mask[(c.maskPos+i)&3]
		}
		c.maskPos = (c.maskPos + n) & 3
	}
	c.remaining -= int64(n)
	if err == io.EOF && n > 0 {
		err = nil
	}
	return n, err
}

// nextFrame reads a frame header. Data frames set remaining, control frames
// are handled here.
func (c *wsConn) nextFrame() error {
	var h [2]byte
	if _, err := io.ReadFull(c.br, h[:]); err != nil {
		return err
	}
	opcode := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	length := int64(h[1] & 0x7f)
	switch length {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(c.br, b[:]); err != nil {
			return err
		}
		length = int64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(c.br, b[:]); err != nil {
			return err
		}
		u := binary.BigEndian.Uint64(b[:])
		if u>>63 != 0 {
			return errors.New("websocket: bad frame length")
		}
		length = int64(u)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, mask[:]); err != nil {
			return err
		}
	}
	switch opcode {
	case 0x0, 0x1, 0x2: // continuation, text, binary
		c.remaining, c.masked, c.mask, c.maskPos = length, masked, mask, 0
		return nil
	case 0x8: // close
		if length > 0 {
			_, _ = io.CopyN(io.Discard, c.br, length)
		}
		c.writeFrame(0x8, nil)
		return io.EOF
	case 0x9, 0xa: // ping, pong
		if length > 125 {
			return errors.New("websocket: oversized control frame")
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(c.br, payload); err != nil {
			return err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i&3]
			}
		}
		if opcode == 0x9 {
			c.writeFrame(0xa, payload)
		}
		return nil
	default:
		return fmt.Errorf("websocket: unknown opcode %d", opcode)
	}
}

func (c *wsConn) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > wsMaxFrame {
			chunk = chunk[:wsMaxFrame]
		}
		if err := c.writeFrame(0x2, chunk); err != nil {
			return written, err
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	if written == 0 {
		// an empty write still has to reach the VLESS layer as "nothing to
		// send", not as an error
		return 0, nil
	}
	return written, nil
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	var head [14]byte
	head[0] = 0x80 | opcode
	n := 2
	maskBit := byte(0)
	if c.client {
		maskBit = 0x80
	}
	switch l := len(payload); {
	case l < 126:
		head[1] = maskBit | byte(l)
	case l <= 0xffff:
		head[1] = maskBit | 126
		binary.BigEndian.PutUint16(head[2:], uint16(l))
		n = 4
	default:
		head[1] = maskBit | 127
		binary.BigEndian.PutUint64(head[2:], uint64(l))
		n = 10
	}
	frame := make([]byte, 0, n+4+len(payload))
	frame = append(frame, head[:n]...)
	if c.client {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		frame = append(frame, mask[:]...)
		start := len(frame)
		frame = append(frame, payload...)
		for i := range payload {
			frame[start+i] ^= mask[i&3]
		}
	} else {
		frame = append(frame, payload...)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed && opcode != 0x8 {
		return net.ErrClosed
	}
	_, err := c.conn.Write(frame)
	return err
}

func (c *wsConn) Close() error {
	c.wmu.Lock()
	already := c.closed
	c.closed = true
	c.wmu.Unlock()
	if !already {
		_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
		c.writeFrame(0x8, nil)
	}
	return c.conn.Close()
}

func (c *wsConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *wsConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c *wsConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *wsConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *wsConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
