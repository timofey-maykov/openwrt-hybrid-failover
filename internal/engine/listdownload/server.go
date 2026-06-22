package listdownload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
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
	go s.serve(runCtx, ln)
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	ln := s.listener
	s.listener = nil
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
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
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method == http.MethodConnect {
		s.handleConnect(conn, br, req)
		return
	}
	s.handleHTTP(conn, req)
}

func (s *Server) outboundTag() string {
	if s.control != nil && s.section != "" {
		return s.control.ActiveTag(s.section)
	}
	return plan.OutboundTag(s.section)
}

func (s *Server) handleConnect(client net.Conn, br *bufio.Reader, req *http.Request) {
	dest := req.Host
	if !strings.Contains(dest, ":") {
		dest += ":443"
	}
	remote, err := s.registry.DialTCP(context.Background(), s.outboundTag(), "tcp", dest)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if br.Buffered() > 0 {
		buf, _ := io.ReadAll(br)
		if len(buf) > 0 {
			_, _ = remote.Write(buf)
		}
	}
	go io.Copy(remote, client)
	_, _ = io.Copy(client, remote)
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
	remote, err := s.registry.DialTCP(context.Background(), s.outboundTag(), "tcp", host)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer remote.Close()
	if err := req.Write(remote); err != nil {
		return
	}
	_, _ = io.Copy(client, remote)
}
