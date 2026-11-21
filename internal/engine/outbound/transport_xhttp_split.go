package outbound

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (h *vlessHandler) openXHTTP(ctx context.Context, rt http.RoundTripper, closer io.Closer, local, remote net.Addr) (net.Conn, error) {
	scheme := "https"
	if h.tls == nil {
		scheme = "http"
	}
	if h.transport.mode == "stream-one" {
		return newXHTTPStream(ctx, rt, closer, local, remote, scheme, h.transport.host, h.transport.path, h.transport.headers)
	}
	return newXHTTPSplit(ctx, rt, closer, local, remote, scheme, h.transport)
}

func xhttpRequest(ctx context.Context, scheme string, t vlessTransport, method, session, seq string, body io.Reader) (*http.Request, error) {
	path := t.path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, scheme+"://"+t.host+path, body)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(req.URL.Path, "/") {
		req.URL.Path += "/"
		if req.URL.RawPath != "" {
			req.URL.RawPath += "/"
		}
	}
	req.URL.Path += session
	if req.URL.RawPath != "" {
		req.URL.RawPath += session
	}
	if seq != "" {
		req.URL.Path += "/" + seq
		if req.URL.RawPath != "" {
			req.URL.RawPath += "/" + seq
		}
	}
	req.Host = t.host
	req.Header.Set("User-Agent", wsDefaultUserAgent)
	for k, v := range t.headers {
		switch http.CanonicalHeaderKey(k) {
		case "Host", "Content-Length", "Transfer-Encoding":
			continue
		}
		req.Header.Set(k, v)
	}
	if method == http.MethodPost && seq == "" {
		req.Header.Set("Content-Type", "application/grpc")
	}
	applyDefaultXPadding(req)
	return req, nil
}

func newXHTTPSplit(ctx context.Context, rt http.RoundTripper, closer io.Closer, local, remote net.Addr, scheme string, t vlessTransport) (net.Conn, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		_ = closer.Close()
		return nil, err
	}
	id[6] = id[6]&15 | 64
	id[8] = id[8]&63 | 128
	session := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	reqCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	out := &xhttpConn{localAddr: local, remoteAddr: remote, ready: make(chan struct{}), cc: closer, cancel: cancel}
	down, err := xhttpRequest(reqCtx, scheme, t, http.MethodGet, session, "", nil)
	if err != nil {
		cancel()
		_ = closer.Close()
		return nil, err
	}
	var upload *http.Request
	if t.mode == "stream-up" {
		pr, pw := io.Pipe()
		out.w = pw
		upload, err = xhttpRequest(reqCtx, scheme, t, http.MethodPost, session, "", pr)
		if err != nil {
			_ = pr.Close()
			_ = out.Close()
			return nil, err
		}
	} else {
		pw := &xhttpPacketWriter{ctx: reqCtx, out: out, queue: make(chan []byte, 8)}
		out.w = pw
		go pw.run(rt, scheme, t, session)
	}
	go func() {
		resp, err := rt.RoundTrip(down)
		if err != nil {
			out.fail(err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			out.fail(fmt.Errorf("xhttp download: HTTP %d", resp.StatusCode))
			return
		}
		out.setReader(resp.Body)
	}()
	if upload != nil {
		go func() {
			resp, err := rt.RoundTrip(upload)
			if err != nil {
				_ = upload.Body.Close()
				out.fail(err)
				return
			}
			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				_ = upload.Body.Close()
				out.fail(fmt.Errorf("xhttp upload: HTTP %d", resp.StatusCode))
				return
			}
			// Closing the response immediately also aborts the streaming upload.
			_, err = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			_ = upload.Body.Close()
			if err == nil {
				err = io.EOF
			}
			out.fail(err)
		}()
	}
	return out, nil
}

// Upload buffering and concurrency are bounded per connection. A serialized
// producer assigns sequence numbers; the server may acknowledge out of order.
type xhttpPacketWriter struct {
	ctx   context.Context
	out   *xhttpConn
	queue chan []byte
	mu    sync.Mutex
}

func (w *xhttpPacketWriter) Close() error { return nil }
func (w *xhttpPacketWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for len(p) > 0 {
		if w.ctx.Err() != nil {
			return n, w.out.writeError()
		}
		size := len(p)
		if size > 32<<10 {
			size = 32 << 10
		}
		b := append([]byte(nil), p[:size]...)
		select {
		case <-w.ctx.Done():
			return n, w.out.writeError()
		case w.queue <- b:
		}
		n += size
		p = p[size:]
	}
	return n, nil
}
func (w *xhttpPacketWriter) run(rt http.RoundTripper, scheme string, t vlessTransport, session string) {
	slots := make(chan struct{}, 4)
	var seq uint64
	var last time.Time
	for {
		var first []byte
		select {
		case <-w.ctx.Done():
			return
		case first = <-w.queue:
		}
		payload := first
	batch:
		for len(payload) < 256<<10 {
			select {
			case b := <-w.queue:
				payload = append(payload, b...)
			default:
				break batch
			}
		}
		if wait := 30*time.Millisecond - time.Since(last); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-w.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		select {
		case slots <- struct{}{}:
		case <-w.ctx.Done():
			return
		}
		current := strconv.FormatUint(seq, 10)
		seq++
		last = time.Now()
		go func(body []byte, seq string) {
			defer func() { <-slots }()
			ctx, cancel := context.WithTimeout(w.ctx, 15*time.Second)
			defer cancel()
			req, err := xhttpRequest(ctx, scheme, t, http.MethodPost, session, seq, bytes.NewReader(body))
			if err != nil {
				w.out.fail(err)
				return
			}
			resp, err := rt.RoundTrip(req)
			if err != nil {
				w.out.fail(err)
				return
			}
			_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("xhttp packet upload: HTTP %d", resp.StatusCode)
			}
			if err != nil {
				w.out.fail(err)
			}
		}(payload, current)
	}
}

func (c *xhttpConn) writeError() error {
	if e := c.err.Load(); e != nil {
		return *e
	}
	return io.ErrClosedPipe
}
