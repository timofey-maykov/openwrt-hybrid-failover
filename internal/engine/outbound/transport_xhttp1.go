package outbound

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

func (h *vlessHandler) dialXHTTP1(ctx context.Context) (net.Conn, error) {
	dial := func(ctx context.Context) (net.Conn, error) {
		c, err := h.dialer.DialContext(ctx, "tcp", h.server)
		if err != nil {
			return nil, err
		}
		return dialTLS(ctx, c, h.tls)
	}
	first, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	local, remote := first.LocalAddr(), first.RemoteAddr()
	var mu sync.Mutex
	next := func(ctx context.Context, _, _ string) (net.Conn, error) {
		mu.Lock()
		c := first
		first = nil
		mu.Unlock()
		if c != nil {
			return c, nil
		}
		return dial(ctx)
	}
	rt := &http.Transport{DisableCompression: true, ForceAttemptHTTP2: false, MaxIdleConnsPerHost: 4, IdleConnTimeout: 90 * time.Second, DialContext: next,
		DialTLSContext:  func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) },
		TLSClientConfig: &tls.Config{NextProtos: []string{"http/1.1"}},
	}
	closer := xhttpCloseFunc(func() error {
		rt.CloseIdleConnections()
		mu.Lock()
		defer mu.Unlock()
		if first != nil {
			return first.Close()
		}
		return nil
	})
	return h.openXHTTP(ctx, rt, closer, local, remote)
}
