package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
)

const defaultListenAddr = "127.0.0.42"
const defaultListenPort = 53

type Plan struct {
	RewriteTTL      int
	Bootstrap       string
	RejectHTTPS     bool
	FakeIPDomains   []string
	ListenAddr      string
	ListenPort      int
}

type Resolver interface {
	ResolveFakeIP(domain string) (string, bool)
}

type Server struct {
	mu       sync.Mutex
	plan     Plan
	resolver Resolver
	udp      *mdns.Server
	tcp      *mdns.Server
	cancel   context.CancelFunc
}

func NewServer(plan Plan, resolver Resolver) *Server {
	if plan.ListenAddr == "" {
		plan.ListenAddr = defaultListenAddr
	}
	if plan.ListenPort == 0 {
		plan.ListenPort = defaultListenPort
	}
	return &Server{plan: plan, resolver: resolver}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udp != nil {
		return nil
	}
	addr := net.JoinHostPort(s.plan.ListenAddr, fmt.Sprintf("%d", s.plan.ListenPort))
	udpConn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("dns udp listen: %w", err)
	}
	tcpLn, err := net.Listen("tcp", addr)
	if err != nil {
		_ = udpConn.Close()
		return fmt.Errorf("dns tcp listen: %w", err)
	}
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handle)
	_, cancel := context.WithCancel(ctx)
	s.udp = &mdns.Server{Net: "udp", Handler: mux, PacketConn: udpConn}
	s.tcp = &mdns.Server{Net: "tcp", Handler: mux, Listener: tcpLn}
	s.cancel = cancel
	go func() { _ = s.udp.ActivateAndServe() }()
	go func() { _ = s.tcp.ActivateAndServe() }()
	// Engine.Runtime.Stop calls Stop; do not also Stop from ctx.Done (re-entrant
	// Shutdown under the same mutex can stall forever and freeze sync/watchdog).
	return waitForListen(addr, 3*time.Second)
}

func waitForListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			_ = conn.Close()
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("listen on %s did not become ready: %w", addr, lastErr)
	}
	return fmt.Errorf("listen on %s did not become ready", addr)
}

func (s *Server) Stop() error {
	s.mu.Lock()
	udp := s.udp
	tcp := s.tcp
	cancel := s.cancel
	s.udp = nil
	s.tcp = nil
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if udp == nil && tcp == nil {
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if udp != nil {
			_ = udp.Shutdown()
		}
		if tcp != nil {
			_ = tcp.Shutdown()
		}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		// Shutdown can block on miekg/dns; release ports so Apply/Run can retry.
		if udp != nil && udp.PacketConn != nil {
			_ = udp.PacketConn.Close()
		}
		if tcp != nil && tcp.Listener != nil {
			_ = tcp.Listener.Close()
		}
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		return fmt.Errorf("dns: shutdown timeout")
	}
}

func (s *Server) handle(w mdns.ResponseWriter, r *mdns.Msg) {
	msg := new(mdns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true
	if len(r.Question) == 0 {
		_ = w.WriteMsg(msg)
		return
	}
	q := r.Question[0]
	if q.Qtype == mdns.TypeHTTPS && s.plan.RejectHTTPS {
		msg.Rcode = mdns.RcodeRefused
		_ = w.WriteMsg(msg)
		return
	}
	// FakeIP / upstream A answers only for A queries. For AAAA return NODATA so clients use IPv4.
	if q.Qtype == mdns.TypeAAAA {
		_ = w.WriteMsg(msg)
		return
	}
	if q.Qtype != mdns.TypeA && q.Qtype != mdns.TypeANY {
		_ = w.WriteMsg(msg)
		return
	}
	name := strings.TrimSuffix(q.Name, ".")
	if ip, ok := s.resolve(name); ok {
		rr := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   q.Name,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
				Ttl:    uint32(s.plan.RewriteTTL),
			},
			A: net.ParseIP(ip).To4(),
		}
		msg.Answer = append(msg.Answer, rr)
	} else if ip, err := s.upstreamLookup(name); err == nil {
		rr := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   q.Name,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
				Ttl:    60,
			},
			A: ip.To4(),
		}
		msg.Answer = append(msg.Answer, rr)
	} else {
		msg.Rcode = mdns.RcodeNameError
	}
	_ = w.WriteMsg(msg)
}

func (s *Server) resolve(name string) (string, bool) {
	if s.resolver == nil {
		return "", false
	}
	if sr, ok := s.resolver.(interface{ ShouldFakeIP(string) bool }); ok {
		if !sr.ShouldFakeIP(name) {
			return "", false
		}
	} else {
		for _, d := range s.plan.FakeIPDomains {
			if strings.EqualFold(name, d) {
				return s.resolver.ResolveFakeIP(name)
			}
		}
		return "", false
	}
	return s.resolver.ResolveFakeIP(name)
}

func (s *Server) upstreamLookup(name string) (net.IP, error) {
	c := mdns.Client{Timeout: 4 * time.Second}
	resp, _, err := c.Exchange(mdnsMsg(name), net.JoinHostPort(s.plan.Bootstrap, "53"))
	if err != nil {
		return nil, err
	}
	for _, ans := range resp.Answer {
		if a, ok := ans.(*mdns.A); ok {
			return a.A, nil
		}
	}
	return nil, fmt.Errorf("no A record")
}

func mdnsMsg(name string) *mdns.Msg {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(name), mdns.TypeA)
	return m
}
