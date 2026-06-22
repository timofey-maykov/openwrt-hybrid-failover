//go:build linux

package tproxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func listenUDPTransparent(port int) (*net.UDPConn, error) {
	addr := &net.UDPAddr{Port: port, IP: net.IPv6zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	file, err := conn.File()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer file.Close()
	fd := int(file.Fd())
	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	_ = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	_ = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
	return conn, nil
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

type udpSession struct {
	remote net.PacketConn
	last   time.Time
}

func (s *Server) serveUDP(ctx context.Context, conn *net.UDPConn) {
	buf := make([]byte, 64*1024)
	oob := make([]byte, 1024)
	var mu sync.Mutex
	sessions := make(map[string]*udpSession)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for key, sess := range sessions {
					if now.Sub(sess.last) > 2*time.Minute {
						_ = sess.remote.Close()
						delete(sessions, key)
					}
				}
				mu.Unlock()
			}
		}
	}()
	for {
		n, oobn, _, clientAddr, err := conn.ReadMsgUDP(buf, oob)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		origDst, err := readOrigDst(oob[:oobn])
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s|%d|%s|%d", clientAddr.IP, clientAddr.Port, origDst.IP, origDst.Port)
		mu.Lock()
		sess, ok := sessions[key]
		if !ok {
			meta := plan.ConnMeta{
				Inbound: "tproxy-in",
				Network: "udp",
				SrcIP:   clientAddr.IP.String(),
				SrcPort: clientAddr.Port,
				DstIP:   origDst.IP.String(),
				DstPort: origDst.Port,
			}
			remote, err := s.router.DialUDP(ctx, meta)
			if err != nil {
				mu.Unlock()
				continue
			}
			sess = &udpSession{remote: remote, last: time.Now()}
			sessions[key] = sess
			go s.relayUDP(ctx, conn, clientAddr, sess, key, &mu, sessions)
		} else {
			sess.last = time.Now()
		}
		mu.Unlock()
		_, _ = sess.remote.WriteTo(buf[:n], origDst)
	}
}

func (s *Server) relayUDP(ctx context.Context, client *net.UDPConn, clientAddr *net.UDPAddr, sess *udpSession, key string, mu *sync.Mutex, sessions map[string]*udpSession) {
	buf := make([]byte, 64*1024)
	for {
		_ = sess.remote.SetReadDeadline(time.Now().Add(2 * time.Minute))
		n, _, err := sess.remote.ReadFrom(buf)
		if err != nil {
			break
		}
		_, _ = client.WriteToUDP(buf[:n], clientAddr)
		sess.last = time.Now()
		select {
		case <-ctx.Done():
			break
		default:
		}
	}
	mu.Lock()
	_ = sess.remote.Close()
	delete(sessions, key)
	mu.Unlock()
}
