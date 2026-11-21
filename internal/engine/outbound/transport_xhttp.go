package outbound

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

// newXHTTPStreamOne opens one bidirectional HTTP/2 stream (Xray "stream-one"):
// POST body is upload, response body is download. mode must already be
// normalized to "stream-one".
func (h *vlessHandler) dialXHTTP2(ctx context.Context, tlsConn net.Conn) (net.Conn, error) {
	cc, err := (&http2.Transport{}).NewClientConn(tlsConn)
	if err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("xhttp: http2 client: %w", err)
	}
	return h.openXHTTP(ctx, cc, cc, tlsConn.LocalAddr(), tlsConn.RemoteAddr())
}

// The POST framing is the same over HTTP/2 and HTTP/3.
func newXHTTPStream(ctx context.Context, rt http.RoundTripper, closer io.Closer, local, remote net.Addr, scheme, host, path string, headers map[string]string) (net.Conn, error) {
	if host == "" {
		_ = closer.Close()
		return nil, fmt.Errorf("xhttp: missing host")
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// The dial context is canceled by the caller after establishing a UDP
	// session. Like net.Dialer, the returned connection lives until Close.
	requestCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	pr, pw := io.Pipe()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, scheme+"://"+host+path, pr)
	if err != nil {
		_ = pw.Close()
		cancel()
		_ = closer.Close()
		return nil, err
	}
	// Xray normalizes the base path with a trailing slash before appending
	// session metadata. Preserve the configured query and encoded path.
	if !strings.HasSuffix(req.URL.Path, "/") {
		req.URL.Path += "/"
		if req.URL.RawPath != "" {
			req.URL.RawPath += "/"
		}
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
		localAddr:  local,
		remoteAddr: remote,
		ready:      make(chan struct{}),
		cc:         closer,
		cancel:     cancel,
	}

	// Return before RoundTrip finishes: VLESS must Write into the POST body
	// while the server may still be sending response headers. Waiting here
	// deadlocks when the peer waits for body before answering.
	go func() {
		resp, err := rt.RoundTrip(req)
		if err != nil {
			_ = pr.Close()
			out.fail(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
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
	w          io.WriteCloser
	r          atomic.Pointer[io.ReadCloser]
	ready      chan struct{}
	readyOnce  sync.Once
	err        atomic.Pointer[error]
	localAddr  net.Addr
	remoteAddr net.Addr

	mu                    sync.Mutex
	cc                    io.Closer
	cancel                context.CancelFunc
	closed                bool
	readTimer, writeTimer *time.Timer
}

func (c *xhttpConn) setReader(rc io.ReadCloser) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		_ = rc.Close()
		return
	}
	c.r.Store(&rc)
	c.readyOnce.Do(func() { close(c.ready) })
}

func (c *xhttpConn) fail(err error) {
	c.err.CompareAndSwap(nil, &err)
	c.readyOnce.Do(func() { close(c.ready) })
	_ = c.Close()
}

func (c *xhttpConn) Read(p []byte) (int, error) {
	rc := c.r.Load()
	if rc == nil {
		select {
		case <-c.ready:
		case <-time.After(30 * time.Second):
			err := fmt.Errorf("xhttp response headers: %w", os.ErrDeadlineExceeded)
			c.fail(err)
			return 0, err
		}
		if e := c.err.Load(); e != nil {
			return 0, *e
		}
		rc = c.r.Load()
		if rc == nil {
			return 0, io.ErrClosedPipe
		}
	}
	n, err := (*rc).Read(p)
	if err != nil {
		if stored := c.err.Load(); stored != nil {
			return n, *stored
		}
	}
	return n, err
}

func (c *xhttpConn) Write(p []byte) (int, error) {
	if err := c.err.Load(); err != nil {
		return 0, *err
	}
	n, err := c.w.Write(p)
	if err != nil {
		if stored := c.err.Load(); stored != nil {
			return n, *stored
		}
	}
	return n, err
}

func (c *xhttpConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	closedErr := error(io.ErrClosedPipe)
	c.err.CompareAndSwap(nil, &closedErr)
	if c.readTimer != nil {
		c.readTimer.Stop()
	}
	if c.writeTimer != nil {
		c.writeTimer.Stop()
	}
	c.cancel()
	c.readyOnce.Do(func() { close(c.ready) })
	_ = c.w.Close()
	if rc := c.r.Load(); rc != nil {
		_ = (*rc).Close()
	}
	if c.cc != nil {
		_ = c.cc.Close()
	}
	return nil
}

func (c *xhttpConn) LocalAddr() net.Addr  { return c.localAddr }
func (c *xhttpConn) RemoteAddr() net.Addr { return c.remoteAddr }
func (c *xhttpConn) setTimer(slot **time.Timer, t time.Time) {
	if *slot != nil {
		(*slot).Stop()
	}
	*slot = nil
	if t.IsZero() {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(time.Until(t), func() {
		c.mu.Lock()
		valid := !c.closed && *slot == timer
		c.mu.Unlock()
		if valid {
			c.fail(os.ErrDeadlineExceeded)
		}
	})
	*slot = timer
}
func (c *xhttpConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.setTimer(&c.readTimer, t)
	c.setTimer(&c.writeTimer, t)
	return nil
}
func (c *xhttpConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.setTimer(&c.readTimer, t)
	return nil
}
func (c *xhttpConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.setTimer(&c.writeTimer, t)
	return nil
}
