package outbound

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// ProbeHTTP verifies the complete outbound path, including deferred proxy
// handshakes, TLS and a successful HTTP response. Merely opening a QUIC stream
// does not prove that the proxy can connect to the requested destination.
func ProbeHTTP(ctx context.Context, h Handler, testURL string) (int, error) {
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return 0, fmt.Errorf("probe URL: %w", err)
	}
	if req.URL.Hostname() == "" || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
		return 0, fmt.Errorf("probe URL must use http or https")
	}
	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	start := time.Now()
	conn, err := h.DialTCP(ctx, "tcp", net.JoinHostPort(req.URL.Hostname(), port))
	if err != nil {
		return 0, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stream := conn
	if req.URL.Scheme == "https" {
		tc := tls.Client(conn, &tls.Config{ServerName: req.URL.Hostname(), NextProtos: []string{"http/1.1"}, MinVersion: tls.VersionTLS12})
		if err := tc.HandshakeContext(ctx); err != nil {
			return 0, fmt.Errorf("TLS: %w", err)
		}
		stream = tc
	}
	req.Close = true
	req.Header.Set("User-Agent", "hybrid-failover-urltest")
	if err := req.Write(stream); err != nil {
		return 0, fmt.Errorf("HTTP write: %w", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(stream, 64<<10)), req)
	if err != nil {
		return 0, fmt.Errorf("HTTP response: %w", err)
	}
	// Close the socket without draining an unbounded body; headers are enough.
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return ms, nil
}
