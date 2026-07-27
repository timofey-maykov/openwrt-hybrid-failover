package tproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

type Router interface {
	Route(meta plan.ConnMeta) (string, error)
	Dial(meta plan.ConnMeta) (net.Conn, error)
	DialUDP(ctx context.Context, meta plan.ConnMeta) (net.PacketConn, error)
}

type Server struct {
	mu          sync.Mutex
	router      Router
	listener    net.Listener
	udpConn     *net.UDPConn
	cancel      context.CancelFunc
	DisableQUIC bool
}

func NewServer(r Router) *Server {
	return &Server{router: r}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	lc := net.ListenConfig{Control: listenTransparent}
	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", plan.TPROXYPort))
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.listener = ln
	s.cancel = cancel
	go s.serve(runCtx, ln)
	if udpConn, err := listenUDPTransparent(plan.TPROXYPort); err == nil {
		s.udpConn = udpConn
		go s.serveUDP(runCtx, udpConn)
	}
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	ln := s.listener
	udpConn := s.udpConn
	s.listener = nil
	s.udpConn = nil
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ln != nil {
		_ = ln.Close()
	}
	if udpConn != nil {
		_ = udpConn.Close()
	}
	return nil
}

func (s *Server) serve(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// Closed listener or transient error: do not busy-spin.
				time.Sleep(20 * time.Millisecond)
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	meta, err := connMeta(conn)
	if err != nil {
		return
	}
	if domain := sniffSNI(br); domain != "" {
		meta.Domain = domain
	}
	_ = conn.SetReadDeadline(time.Time{})
	remote, err := s.router.Dial(meta)
	if err != nil {
		return
	}
	defer remote.Close()
	relay(br, conn, remote)
}

func connMeta(conn net.Conn) (plan.ConnMeta, error) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return plan.ConnMeta{}, fmt.Errorf("not tcp")
	}
	meta := plan.ConnMeta{Inbound: "tproxy-in", Network: "tcp"}
	if ra := tcp.RemoteAddr(); ra != nil {
		host, portStr, _ := net.SplitHostPort(ra.String())
		meta.SrcIP = host
		fmt.Sscanf(portStr, "%d", &meta.SrcPort)
	}
	if orig, err := originalDst(tcp); err == nil {
		meta.DstIP = orig.IP.String()
		meta.DstPort = orig.Port
	} else if la := tcp.LocalAddr(); la != nil {
		host, portStr, _ := net.SplitHostPort(la.String())
		meta.DstIP = host
		fmt.Sscanf(portStr, "%d", &meta.DstPort)
	}
	return meta, nil
}

func relay(client *bufio.Reader, conn net.Conn, remote net.Conn) {
	done := make(chan struct{}, 2)
	go copyReaderToConn(remote, client, done)
	go copyConnToConn(conn, remote, done)
	<-done
}

func copyReaderToConn(dst net.Conn, src io.Reader, done chan struct{}) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	done <- struct{}{}
}

func copyConnToConn(dst, src net.Conn, done chan struct{}) {
	_, _ = io.Copy(dst, src)
	_ = src.Close()
	done <- struct{}{}
}

type originalDest struct {
	IP   net.IP
	Port int
}

func originalDst(conn *net.TCPConn) (originalDest, error) {
	return originalDstPlatform(conn)
}
