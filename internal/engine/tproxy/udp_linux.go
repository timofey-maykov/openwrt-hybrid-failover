//go:build linux

package tproxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func udpSocketControl(receiveOriginal bool) func(string, string, syscall.RawConn) error {
	return func(network, address string, raw syscall.RawConn) error {
		var optionErr error
		err := raw.Control(func(fd uintptr) {
			options := [][3]int{{syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1}, {syscall.SOL_IP, syscall.IP_TRANSPARENT, 1}}
			if receiveOriginal {
				options = append(options, [3]int{syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1})
			}
			for _, option := range options {
				if optionErr = syscall.SetsockoptInt(int(fd), option[0], option[1], option[2]); optionErr != nil {
					return
				}
			}
		})
		if err != nil {
			return err
		}
		return optionErr
	}
}

func listenUDPTransparent(port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: udpSocketControl(true)}
	conn, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}
	return conn.(*net.UDPConn), nil
}

// Replies must use the destination the client originally addressed, including
// its port and FakeIP. A write on the shared :1602 listener changes the source.
func dialUDPReply(ctx context.Context, original, client *net.UDPAddr) (net.Conn, error) {
	d := net.Dialer{LocalAddr: original, Control: udpSocketControl(false)}
	return d.DialContext(ctx, "udp4", client.String())
}

func readOrigDst(oob []byte) (*net.UDPAddr, error) {
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgs {
		if msg.Header.Level == syscall.SOL_IP && msg.Header.Type == syscall.IP_RECVORIGDSTADDR && len(msg.Data) >= 8 {
			port := int(binary.BigEndian.Uint16(msg.Data[2:4]))
			ip := net.IPv4(msg.Data[4], msg.Data[5], msg.Data[6], msg.Data[7])
			return &net.UDPAddr{IP: ip, Port: port}, nil
		}
	}
	return nil, fmt.Errorf("original destination missing")
}

var udpRelayBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 32*1024)
		return &b
	},
}

func acquireUDPRelayBuf() []byte {
	if v := udpRelayBufPool.Get(); v != nil {
		return *v.(*[]byte)
	}
	b := make([]byte, 32*1024)
	return b
}

func releaseUDPRelayBuf(buf []byte) {
	if cap(buf) < 32*1024 || cap(buf) > 64*1024 {
		return
	}
	udpRelayBufPool.Put(&buf)
}

type udpSession struct {
	mu          sync.Mutex
	remote      net.PacketConn
	reply       net.Conn
	established bool
	failed      bool
	pending     [][]byte
	last        time.Time // protected by the session map mutex
	closeOnce   sync.Once
}

func (s *udpSession) close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		remote, reply := s.remote, s.reply
		s.mu.Unlock()
		if remote != nil {
			_ = remote.Close()
		}
		if reply != nil {
			_ = reply.Close()
		}
	})
}

// markFailed records that dialing the outbound never completed, so any
// queued packets are dropped and future writes are no-ops.
func (s *udpSession) markFailed() {
	s.mu.Lock()
	s.failed = true
	s.pending = nil
	s.mu.Unlock()
}

// establish attaches the dialed connections and flushes any packets that
// arrived on the client socket while the dial was still in flight.
func (s *udpSession) establish(remote net.PacketConn, reply net.Conn, origDst *net.UDPAddr) {
	s.mu.Lock()
	s.remote = remote
	s.reply = reply
	pending := s.pending
	s.pending = nil
	s.established = true
	s.mu.Unlock()
	for _, p := range pending {
		_ = remote.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := remote.WriteTo(p, origDst); err != nil {
			s.close()
			return
		}
	}
}

// enqueueOrSend writes payload immediately once the session is established,
// or buffers it (bounded) while the outbound dial is still in progress.
func (s *udpSession) enqueueOrSend(payload []byte, origDst *net.UDPAddr) {
	s.mu.Lock()
	if s.established {
		remote := s.remote
		s.mu.Unlock()
		_ = remote.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := remote.WriteTo(payload, origDst); err != nil {
			s.close()
		}
		return
	}
	if s.failed {
		s.mu.Unlock()
		return
	}
	const maxPending = 64
	if len(s.pending) < maxPending {
		s.pending = append(s.pending, payload)
	}
	s.mu.Unlock()
}

func (s *Server) serveUDP(ctx context.Context, conn *net.UDPConn) {
	buf := make([]byte, 64*1024)
	oob := make([]byte, 1024)
	var mu sync.Mutex
	sessions := make(map[string]*udpSession)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				_ = conn.Close()
				mu.Lock()
				for key, sess := range sessions {
					sess.close()
					delete(sessions, key)
				}
				mu.Unlock()
				return
			case <-ticker.C:
				mu.Lock()
				for key, sess := range sessions {
					if time.Since(sess.last) > 2*time.Minute {
						sess.close()
						delete(sessions, key)
					}
				}
				mu.Unlock()
			}
		}
	}()
	for {
		n, oobn, flags, clientAddr, err := conn.ReadMsgUDP(buf, oob)
		if err != nil {
			if runCtx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if flags&(syscall.MSG_TRUNC|syscall.MSG_CTRUNC) != 0 {
			continue
		}
		origDst, err := readOrigDst(oob[:oobn])
		if err != nil {
			continue
		}
		if s.DisableQUIC && origDst.Port == 443 {
			continue
		}
		key := clientAddr.String() + "|" + origDst.String()
		payload := append([]byte(nil), buf[:n]...)
		mu.Lock()
		sess := sessions[key]
		mu.Unlock()
		if sess == nil {
			meta := plan.ConnMeta{Inbound: "tproxy-in", Network: "udp", SrcIP: clientAddr.IP.String(), SrcPort: clientAddr.Port, DstIP: origDst.IP.String(), DstPort: origDst.Port}
			sess = &udpSession{last: time.Now(), pending: [][]byte{payload}}
			mu.Lock()
			if runCtx.Err() != nil {
				mu.Unlock()
				continue
			}
			sessions[key] = sess
			mu.Unlock()
			// Dialing the outbound (and the reply socket) can block for up to
			// 10s; doing it off this goroutine keeps unrelated UDP flows
			// flowing through the shared TPROXY listener while it happens.
			go s.establishUDPSession(runCtx, sess, meta, origDst, clientAddr, key, &mu, sessions)
			continue
		}
		mu.Lock()
		sess.last = time.Now()
		mu.Unlock()
		sess.enqueueOrSend(payload, origDst)
	}
}

func (s *Server) establishUDPSession(runCtx context.Context, sess *udpSession, meta plan.ConnMeta, origDst, clientAddr *net.UDPAddr, key string, mu *sync.Mutex, sessions map[string]*udpSession) {
	dialCtx, dialCancel := context.WithTimeout(runCtx, 10*time.Second)
	defer dialCancel()
	remote, err := s.router.DialUDP(dialCtx, meta)
	if err != nil {
		sess.markFailed()
		mu.Lock()
		if sessions[key] == sess {
			delete(sessions, key)
		}
		mu.Unlock()
		return
	}
	reply, err := dialUDPReply(dialCtx, origDst, clientAddr)
	if err != nil {
		_ = remote.Close()
		sess.markFailed()
		mu.Lock()
		if sessions[key] == sess {
			delete(sessions, key)
		}
		mu.Unlock()
		return
	}

	mu.Lock()
	stillCurrent := sessions[key] == sess && runCtx.Err() == nil
	mu.Unlock()
	if !stillCurrent {
		_ = remote.Close()
		_ = reply.Close()
		return
	}

	sess.establish(remote, reply, origDst)
	go s.relayUDP(runCtx, sess, key, mu, sessions)
}

func (s *Server) relayUDP(ctx context.Context, sess *udpSession, key string, mu *sync.Mutex, sessions map[string]*udpSession) {
	buf := acquireUDPRelayBuf()
	defer releaseUDPRelayBuf(buf)
	defer func() {
		sess.close()
		mu.Lock()
		if sessions[key] == sess {
			delete(sessions, key)
		}
		mu.Unlock()
	}()
	for ctx.Err() == nil {
		_ = sess.remote.SetReadDeadline(time.Now().Add(2 * time.Minute))
		n, _, err := sess.remote.ReadFrom(buf)
		if err != nil {
			return
		}
		_ = sess.reply.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err = sess.reply.Write(buf[:n]); err != nil {
			return
		}
		mu.Lock()
		sess.last = time.Now()
		mu.Unlock()
	}
}
