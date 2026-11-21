//go:build with_utls

package outbound

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-vmess/vless"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type visionDirectLogger struct {
	nopLogger
	reads atomic.Int32
}

func (l *visionDirectLogger) Trace(args ...any) {
	if strings.Contains(fmt.Sprint(args...), "XtlsRead readV") {
		l.reads.Add(1)
	}
}

type visionTLSService struct {
	echoServer
	cert tls.Certificate
}

func (s visionTLSService) NewConnectionEx(ctx context.Context, c net.Conn, source, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		defer c.Close()
		inner := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS13})
		_, err := io.Copy(inner, inner)
		if onClose != nil {
			onClose(err)
		}
	}()
}

func TestVisionNestedTLSDirectRoundTrip(t *testing.T) {
	cert := selfSigned(t)
	log := &visionDirectLogger{}
	svc := vless.NewService[int](log, visionTLSService{cert: cert})
	svc.UpdateUsers([]int{0}, []string{vlessTestUUID}, []string{vless.FlowVision})
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				if err := svc.NewConnection(context.Background(), c, M.SocksaddrFromNet(c.RemoteAddr()), nil); err != nil {
					_ = c.Close()
				}
			}()
		}
	}()
	for _, fp := range []string{"none", "chrome", "firefox"} {
		t.Run(fp, func(t *testing.T) {
			h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@%s?security=tls&sni=localhost&allowInsecure=1&fp=%s&flow=xtls-rprx-vision", vlessTestUUID, ln.Addr(), fp)).(*vlessHandler)
			h.client, err = vless.NewClient(vlessTestUUID, vless.FlowVision, log)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, err := h.DialTCP(ctx, "tcp", "inner.example:443")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			inner := tls.Client(c, &tls.Config{ServerName: "localhost", InsecureSkipVerify: true, MinVersion: tls.VersionTLS13})
			if err := inner.HandshakeContext(ctx); err != nil {
				t.Fatal(err)
			}
			before := log.reads.Load()
			payload := strings.Repeat("vision direct data\n", 8192)
			done := make(chan error, 1)
			go func() { _, err := io.WriteString(inner, payload); done <- err }()
			got := make([]byte, len(payload))
			if _, err := io.ReadFull(inner, got); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if string(got) != payload {
				t.Fatal("payload corrupted")
			}
			if log.reads.Load() <= before {
				t.Fatal("Vision did not drain TLS buffers for direct mode")
			}
		})
	}
}
