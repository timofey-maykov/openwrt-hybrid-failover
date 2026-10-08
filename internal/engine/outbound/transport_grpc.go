package outbound

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// The VLESS grpc transport as xray and sing-box speak it ("gun"): one HTTP/2
// POST to /<serviceName>/Tun whose request and response bodies are streams of
// gRPC messages, each message a protobuf with the bytes in field 1. It needs
// golang.org/x/net/http2 only, not the whole gRPC library, which would add
// several megabytes to a binary that has to fit in the flash of a router.

const (
	grpcDefaultService = "GunService"
	grpcChunk          = 1 << 15
	grpcMaxMessage     = 1 << 24
	grpcUserAgent      = "grpc-go/1.65.0"
)

type grpcOptions struct {
	service   string
	authority string
	// keepalive pings while the stream is idle, zero for none
	idleTimeout time.Duration
	pingTimeout time.Duration
}

// newGRPCClient starts the stream over conn, which must already be TLS with
// "h2" negotiated (or a plain connection to a server that speaks HTTP/2 from
// the start).
func newGRPCClient(conn net.Conn, serverName string, opt grpcOptions) (net.Conn, error) {
	service := strings.Trim(opt.service, "/")
	if service == "" {
		service = grpcDefaultService
	}
	authority := opt.authority
	if authority == "" {
		authority = serverName
	}
	tr := &http2.Transport{ReadIdleTimeout: opt.idleTimeout, PingTimeout: opt.pingTimeout}
	cc, err := tr.NewClientConn(conn)
	if err != nil {
		return nil, fmt.Errorf("grpc: %w", err)
	}
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	req := (&http.Request{
		Method:        http.MethodPost,
		URL:           &url.URL{Scheme: "https", Host: authority, Path: "/" + service + "/Tun"},
		Host:          authority,
		Header:        http.Header{"Content-Type": {"application/grpc"}, "Te": {"trailers"}, "User-Agent": {grpcUserAgent}},
		Body:          pr,
		ContentLength: -1,
	}).WithContext(ctx)

	g := &grpcConn{
		raw: conn, cc: cc, pw: pw, pr: pr, cancel: cancel,
		data: make(chan []byte, 8), ready: make(chan struct{}),
	}
	go g.run(req)
	return g, nil
}

type grpcConn struct {
	raw    net.Conn
	cc     *http2.ClientConn
	pw     *io.PipeWriter
	pr     *io.PipeReader
	cancel context.CancelFunc

	ready chan struct{} // closed once the response headers are in (or failed)
	data  chan []byte   // decoded payloads, closed at the end of the stream
	rerr  error         // why data was closed, set before it is closed

	rmu  sync.Mutex
	left []byte

	wmu sync.Mutex

	dmu       sync.Mutex
	rdeadline time.Time
	wdeadline time.Time

	closeOnce sync.Once
}

func (g *grpcConn) run(req *http.Request) {
	resp, err := g.cc.RoundTrip(req)
	if err != nil {
		g.rerr = fmt.Errorf("grpc: %w", err)
		close(g.ready)
		close(g.data)
		return
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		g.rerr = fmt.Errorf("grpc: server answered %s", resp.Status)
		close(g.ready)
		close(g.data)
		return
	}
	close(g.ready)
	defer resp.Body.Close()
	for {
		payload, err := readGRPCMessage(resp.Body)
		if err != nil {
			g.rerr = err
			close(g.data)
			return
		}
		if len(payload) == 0 {
			continue
		}
		select {
		case g.data <- payload:
		case <-req.Context().Done():
			g.rerr = net.ErrClosed
			close(g.data)
			return
		}
	}
}

// readGRPCMessage reads one length-prefixed message and returns the bytes of
// field 1 of the protobuf inside.
func readGRPCMessage(r io.Reader) ([]byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if hdr[0] != 0 {
		return nil, errors.New("grpc: compressed messages are not supported")
	}
	n := binary.BigEndian.Uint32(hdr[1:])
	if n > grpcMaxMessage {
		return nil, errors.New("grpc: message too large")
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, err
	}
	return decodeHunk(msg)
}

// decodeHunk takes the bytes of field 1 (wire type 2) out of a protobuf.
func decodeHunk(msg []byte) ([]byte, error) {
	var out []byte
	for len(msg) > 0 {
		tag, n := binary.Uvarint(msg)
		if n <= 0 {
			return nil, errors.New("grpc: bad protobuf")
		}
		msg = msg[n:]
		switch tag & 7 {
		case 2:
			l, n := binary.Uvarint(msg)
			if n <= 0 || uint64(len(msg)-n) < l {
				return nil, errors.New("grpc: bad protobuf")
			}
			if tag>>3 == 1 {
				out = append(out, msg[n:n+int(l)]...)
			}
			msg = msg[n+int(l):]
		case 0:
			_, n := binary.Uvarint(msg)
			if n <= 0 {
				return nil, errors.New("grpc: bad protobuf")
			}
			msg = msg[n:]
		default:
			return nil, errors.New("grpc: unexpected protobuf field")
		}
	}
	return out, nil
}

func encodeGRPCMessage(p []byte) []byte {
	var vl [binary.MaxVarintLen64]byte
	vn := binary.PutUvarint(vl[:], uint64(len(p)))
	protoLen := 1 + vn + len(p)
	out := make([]byte, 0, 5+protoLen)
	out = append(out, 0)
	out = binary.BigEndian.AppendUint32(out, uint32(protoLen))
	out = append(out, 0x0a)
	out = append(out, vl[:vn]...)
	return append(out, p...)
}

func (g *grpcConn) Read(p []byte) (int, error) {
	g.rmu.Lock()
	defer g.rmu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if len(g.left) == 0 {
		var timeout <-chan time.Time
		g.dmu.Lock()
		dl := g.rdeadline
		g.dmu.Unlock()
		if !dl.IsZero() {
			d := time.Until(dl)
			if d <= 0 {
				return 0, os.ErrDeadlineExceeded
			}
			t := time.NewTimer(d)
			defer t.Stop()
			timeout = t.C
		}
		select {
		case payload, ok := <-g.data:
			if !ok {
				<-g.ready
				if g.rerr == nil {
					return 0, io.EOF
				}
				return 0, g.rerr
			}
			g.left = payload
		case <-timeout:
			return 0, os.ErrDeadlineExceeded
		}
	}
	n := copy(p, g.left)
	g.left = g.left[n:]
	return n, nil
}

func (g *grpcConn) Write(p []byte) (int, error) {
	g.wmu.Lock()
	defer g.wmu.Unlock()
	written := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > grpcChunk {
			chunk = chunk[:grpcChunk]
		}
		if err := g.writeBody(encodeGRPCMessage(chunk)); err != nil {
			return written, err
		}
		written += len(chunk)
		p = p[len(chunk):]
	}
	return written, nil
}

// writeBody puts a frame into the request body, giving up at the write
// deadline. The pipe blocks until HTTP/2 has room, so it runs on its own
// goroutine when there is a deadline to keep.
func (g *grpcConn) writeBody(frame []byte) error {
	g.dmu.Lock()
	dl := g.wdeadline
	g.dmu.Unlock()
	if dl.IsZero() {
		_, err := g.pw.Write(frame)
		return err
	}
	d := time.Until(dl)
	if d <= 0 {
		return os.ErrDeadlineExceeded
	}
	done := make(chan error, 1)
	go func() {
		_, err := g.pw.Write(frame)
		done <- err
	}()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case err := <-done:
		return err
	case <-t.C:
		return os.ErrDeadlineExceeded
	}
}

func (g *grpcConn) Close() error {
	g.closeOnce.Do(func() {
		_ = g.pw.Close()
		g.cancel()
		_ = g.pr.Close()
		_ = g.cc.Close()
		_ = g.raw.Close()
	})
	return nil
}

func (g *grpcConn) LocalAddr() net.Addr  { return g.raw.LocalAddr() }
func (g *grpcConn) RemoteAddr() net.Addr { return g.raw.RemoteAddr() }

func (g *grpcConn) SetDeadline(t time.Time) error {
	_ = g.SetReadDeadline(t)
	return g.SetWriteDeadline(t)
}

func (g *grpcConn) SetReadDeadline(t time.Time) error {
	g.dmu.Lock()
	g.rdeadline = t
	g.dmu.Unlock()
	return nil
}

func (g *grpcConn) SetWriteDeadline(t time.Time) error {
	g.dmu.Lock()
	g.wdeadline = t
	g.dmu.Unlock()
	return nil
}
