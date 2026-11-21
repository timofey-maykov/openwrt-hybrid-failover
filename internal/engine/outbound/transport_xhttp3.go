package outbound

import (
	"context"
	"fmt"
	"net"
	"time"

	quic "github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
)

type xhttpCloseFunc func() error

func (f xhttpCloseFunc) Close() error { return f() }

func (h *vlessHandler) dialXHTTP3(ctx context.Context) (net.Conn, error) {
	cfg, err := h.tls.Config()
	if err != nil {
		return nil, fmt.Errorf("xhttp HTTP/3: TLS config: %w", err)
	}
	// Use the engine dialer so interface binding and the bypass socket mark
	// apply to QUIC too. The HTTP Host may differ from the server address.
	udp, err := h.dialer.DialContext(ctx, "udp", h.server)
	if err != nil {
		return nil, err
	}
	pc := &connectedPacketConn{Conn: udp}
	tr := &quic.Transport{Conn: pc}
	conn, err := tr.Dial(ctx, udp.RemoteAddr(), cfg.Clone(), &quic.Config{
		HandshakeIdleTimeout:       8 * time.Second,
		MaxIdleTimeout:             90 * time.Second,
		KeepAlivePeriod:            30 * time.Second,
		MaxStreamReceiveWindow:     1 << 20,
		MaxConnectionReceiveWindow: 4 << 20,
	})
	if err != nil {
		_ = pc.Close()
		_ = tr.Close()
		return nil, fmt.Errorf("xhttp HTTP/3: %w", err)
	}
	closer := xhttpCloseFunc(func() error {
		_ = conn.CloseWithError(0, "")
		// Unblock the packet reader before waiting for the QUIC transport.
		_ = pc.Close()
		return tr.Close()
	})
	cc := (&http3.Transport{DisableCompression: true}).NewClientConn(conn)
	return newXHTTPStream(ctx, cc, closer, udp.LocalAddr(), udp.RemoteAddr(), h.transport.host, h.transport.path, h.transport.headers)
}
