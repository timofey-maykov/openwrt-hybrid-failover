package outbound

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

// newXHTTPStreamOne opens one bidirectional HTTP/2 stream (Xray "stream-one"):
// POST body is upload, response body is download. mode must already be
// normalized to "stream-one".
func newXHTTPStreamOne(ctx context.Context, tlsConn net.Conn, host, path string, headers map[string]string) (net.Conn, error) {
	_ = ctx
	if host == "" {
		return nil, fmt.Errorf("xhttp: missing host")
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	cc, err := (&http2.Transport{}).NewClientConn(tlsConn)
	if err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("xhttp: http2 client: %w", err)
	}

	pr, pw := io.Pipe()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://"+host+path, pr)
	if err != nil {
		_ = pw.Close()
		_ = cc.Close()
		return nil, err
	}
	req.Host = host
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("User-Agent", wsDefaultUserAgent)
	for k, v := range headers {
		switch http.CanonicalHeaderKey(k) {
		case "Host", "Content-Type", "Content-Length", "Transfer-Encoding":
			continue
		case "User-Agent":
			req.Header.Set(k, v)
			continue
		}
		req.Header.Set(k, v)
	}
	applyDefaultXPadding(req)

	out := &xhttpConn{
		w:          pw,
		localAddr:  tlsConn.LocalAddr(),
		remoteAddr: tlsConn.RemoteAddr(),
		ready:      make(chan struct{}),
		cc:         cc,
	}

	// Return before RoundTrip finishes: VLESS must Write into the POST body
	// while the server may still be sending response headers. Waiting here
	// deadlocks when the peer waits for body before answering.
	go func() {
		resp, err := cc.RoundTrip(req)
		if err != nil {
			_ = pr.Close()
			out.fail(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			_ = pr.Close()
			out.fail(fmt.Errorf("xhttp: unexpected status %s", resp.Status))
			return
		}
		out.setReader(resp.Body)
	}()

	return out, nil
}

func applyDefaultXPadding(req *http.Request) {
	// Xray default: x_padding query stuffed into the Referer header (100..1000).
	var b [2]byte
	_, _ = rand.Read(b[:])
	n := 100 + (int(b[0])<<8|int(b[1]))%901
	pad := strings.Repeat("X", n)
	u := *req.URL
	u.RawQuery = "x_padding=" + pad
	req.Header.Set("Referer", u.String())
}

type xhttpConn struct {
	w          *io.PipeWriter
	r          atomic.Pointer[io.ReadCloser]
	ready      chan struct{}
	readyOnce  sync.Once
	err        atomic.Pointer[error]
	localAddr  net.Addr
	remoteAddr net.Addr

	mu     sync.Mutex
	cc     *http2.ClientConn
	closed bool
}

func (c *xhttpConn) setReader(rc io.ReadCloser) {
	c.r.Store(&rc)
	c.readyOnce.Do(func() { close(c.ready) })
}

func (c *xhttpConn) fail(err error) {
	c.err.Store(&err)
	c.readyOnce.Do(func() { close(c.ready) })
}

func (c *xhttpConn) Read(p []byte) (int, error) {
	rc := c.r.Load()
	if rc == nil {
		select {
		case <-c.ready:
		case <-time.After(30 * time.Second):
			return 0, fmt.Errorf("xhttp: timed out waiting for response headers")
		}
		if e := c.err.Load(); e != nil {
			return 0, *e
		}
		rc = c.r.Load()
		if rc == nil {
			return 0, io.ErrClosedPipe
		}
	}
	return (*rc).Read(p)
}

func (c *xhttpConn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func (c *xhttpConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	_ = c.w.Close()
	if rc := c.r.Load(); rc != nil {
		_ = (*rc).Close()
	}
	if c.cc != nil {
		_ = c.cc.Close()
	}
	return nil
}

func (c *xhttpConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *xhttpConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *xhttpConn) SetDeadline(t time.Time) error      { return nil }
func (c *xhttpConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *xhttpConn) SetWriteDeadline(t time.Time) error { return nil }
