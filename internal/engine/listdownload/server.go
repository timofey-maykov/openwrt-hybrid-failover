package listdownload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
)

const defaultPort = 1610

type Server struct {
	mu       sync.Mutex
	section  string
	port     int
	registry *outbound.Registry
	control  *control.Control
	listener net.Listener
	cancel   context.CancelFunc
	channels []string // non-nil for the subscription-only proxy
	conns    map[net.Conn]struct{}
	stopped  bool
}

// NewSubscription exposes existing channels, not section selectors which may
// silently fall back to WAN. It is available independently of list downloads.
func NewSubscription(channels []plan.ChannelPlan, reg *outbound.Registry) *Server {
	s := New("", netfetch.SubscriptionProxyPort, reg, nil)
	s.channels = make([]string, 0, len(channels))
	seen := make(map[string]bool)
	for _, ch := range channels {
		if ch.Tag != "" && !seen[ch.Tag] {
			s.channels = append(s.channels, ch.Tag)
			seen[ch.Tag] = true
		}
	}
	return s
}

func New(section string, port int, reg *outbound.Registry, ctrl *control.Control) *Server {
	if port <= 0 {
		port = defaultPort
	}
	return &Server{section: section, port: port, registry: reg, control: ctrl}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.listener = ln
	s.cancel = cancel
	s.stopped = false
	go s.serve(runCtx, ln)
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	ln := s.listener
	s.listener = nil
	s.cancel = nil
	s.stopped = true
	if cancel != nil {
		cancel()
	}
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
}

func (s *Server) serve(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}
		s.mu.Lock()
		if ctx.Err() != nil || s.stopped {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		if s.conns == nil {
			s.conns = make(map[net.Conn]struct{})
		}
		s.conns[conn] = struct{}{}
		s.mu.Unlock()
		go func() {
			defer func() { s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock() }()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if s.channels != nil {
		_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	}
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	defer req.Body.Close()
	if s.channels != nil {
		if req.Method == http.MethodGet && !req.URL.IsAbs() && req.URL.Path == "/channels" {
			body, _ := json.Marshal(s.channels)
			_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
			return
		}
		tag := req.Header.Get(netfetch.SubscriptionChannelHeader)
		found := false
		for _, channel := range s.channels {
			found = found || tag == channel
		}
		if !found {
			_, _ = conn.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
			return
		}
	}
	if req.Method == http.MethodConnect {
		s.handleConnect(conn, br, req)
		return
	}
	s.handleHTTP(conn, req)
}

func (s *Server) outboundTag(req *http.Request) string {
	if s.channels != nil {
		return req.Header.Get(netfetch.SubscriptionChannelHeader)
	}
	if s.control != nil && s.section != "" {
		return s.control.ActiveTag(s.section)
	}
	return plan.OutboundTag(s.section)
}

// Subscription attempts have a bounded dial; list downloads retain their
// existing lifetime and may legitimately need longer on slow links.
func (s *Server) dialContext() (context.Context, context.CancelFunc) {
	if s.channels != nil {
		return context.WithTimeout(context.Background(), 15*time.Second)
	}
	return context.WithCancel(context.Background())
}

func (s *Server) handleConnect(client net.Conn, br *bufio.Reader, req *http.Request) {
	dest := req.Host
	if !strings.Contains(dest, ":") {
		dest += ":443"
	}
	ctx, cancel := s.dialContext()
	defer cancel()
	remote, err := s.registry.DialTCP(ctx, s.outboundTag(req), "tcp", dest)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	if !s.trackRemote(remote) {
		return
	}
	defer func() { s.mu.Lock(); delete(s.conns, remote); s.mu.Unlock() }()
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if br.Buffered() > 0 {
		buf := make([]byte, br.Buffered())
		_, _ = io.ReadFull(br, buf)
		if len(buf) > 0 {
			_, _ = remote.Write(buf)
		}
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(remote, client)
		_ = remote.Close()
		close(done)
	}()
	_, _ = io.Copy(client, remote)
	_ = client.Close()
	_ = remote.Close()
	<-done
}

func (s *Server) handleHTTP(client net.Conn, req *http.Request) {
	if req.URL == nil {
		return
	}
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	if !strings.Contains(host, ":") {
		if req.URL.Scheme == "https" || strings.HasSuffix(host, ":443") {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	ctx, cancel := s.dialContext()
	defer cancel()
	remote, err := s.registry.DialTCP(ctx, s.outboundTag(req), "tcp", host)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	if !s.trackRemote(remote) {
		return
	}
	defer func() { s.mu.Lock(); delete(s.conns, remote); s.mu.Unlock() }()
	req.Header.Del(netfetch.SubscriptionChannelHeader)
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Proxy-Connection")
	req.Close = true // one request per accepted connection; don't wait on keep-alive
	if err := req.Write(remote); err != nil {
		return
	}
	_, _ = io.Copy(client, remote)
}

// Track both halves so Stop also interrupts a panel that never responds.
func (s *Server) trackRemote(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	if s.conns == nil {
		s.conns = make(map[net.Conn]struct{})
	}
	s.conns[conn] = struct{}{}
	if s.channels != nil {
		_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	}
	return true
}
