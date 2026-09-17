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
	remote    net.PacketConn
	reply     net.Conn
	last      time.Time // protected by the session map mutex
	closeOnce sync.Once
}

func (s *udpSession) close() {
	s.closeOnce.Do(func() {
		_ = s.remote.Close()
		_ = s.reply.Close()
	})
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
		mu.Lock()
		sess := sessions[key]
		mu.Unlock()
		if sess == nil {
			meta := plan.ConnMeta{Inbound: "tproxy-in", Network: "udp", SrcIP: clientAddr.IP.String(), SrcPort: clientAddr.Port, DstIP: origDst.IP.String(), DstPort: origDst.Port}
			dialCtx, dialCancel := context.WithTimeout(runCtx, 10*time.Second)
			remote, err := s.router.DialUDP(dialCtx, meta)
			if err != nil {
				dialCancel()
				continue
			}
			reply, err := dialUDPReply(dialCtx, origDst, clientAddr)
			dialCancel()
			if err != nil {
				_ = remote.Close()
				continue
			}
			sess = &udpSession{remote: remote, reply: reply, last: time.Now()}
			mu.Lock()
			if runCtx.Err() != nil {
				mu.Unlock()
				sess.close()
				return
			}
			sessions[key] = sess
			mu.Unlock()
			go s.relayUDP(runCtx, sess, key, &mu, sessions)
		}
		mu.Lock()
		sess.last = time.Now()
		mu.Unlock()
		_ = sess.remote.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := sess.remote.WriteTo(buf[:n], origDst); err != nil {
			sess.close()
		}
	}
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
