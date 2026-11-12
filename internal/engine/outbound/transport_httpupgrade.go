package outbound

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// newHTTPUpgradeClient does an HTTP Upgrade that looks like WebSocket, then
// returns the raw byte stream (no WebSocket framing). That is what Xray and
// sing-box call "httpupgrade".
func newHTTPUpgradeClient(ctx context.Context, conn net.Conn, host, path string, headers map[string]string) (net.Conn, error) {
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
	req.WriteString("Connection: Upgrade\r\nUpgrade: websocket\r\n")
	fmt.Fprintf(&req, "Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n", key)
	userAgentSet := false
	for k, v := range headers {
		switch http.CanonicalHeaderKey(k) {
		case "Host", "Upgrade", "Connection", "Sec-Websocket-Key", "Sec-Websocket-Version":
			continue
		case "User-Agent":
			userAgentSet = true
		}
		if strings.ContainsAny(k, "\r\n:") || strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("httpupgrade: bad header in the link")
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
		return nil, fmt.Errorf("httpupgrade handshake: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols ||
		!strings.EqualFold(resp.Header.Get("Connection"), "upgrade") ||
		!strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("httpupgrade handshake: server answered %s", resp.Status)
	}
	if br.Buffered() > 0 {
		return &bufConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

// bufConn reads leftover handshake bytes before the underlying conn.
type bufConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
