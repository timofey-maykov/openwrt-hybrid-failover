//go:build with_utls

package outbound

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	utls "github.com/metacubex/utls"
	"github.com/sagernet/sing-vmess/vless"
	M "github.com/sagernet/sing/common/metadata"
)

// VLESS with flow xtls-rprx-vision reaches into the TLS connection by type.
// sing-vmess knows uTLS connections only when it is built with the with_utls
// tag; without it the REALITY connection is rejected with "vision: not a valid
// supported TLS connection: *outbound.realityConnWrapper".
func TestVisionAcceptsRealityConnection(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "example.com"}, utls.HelloChrome_Auto)
	w := &realityConnWrapper{UConn: uc}
	var id [16]byte
	if _, err := vless.NewVisionConn(w, w, id, newNopLogger()); err != nil {
		t.Fatalf("vision refuses the REALITY connection: %v", err)
	}
}

func TestVisionAcceptsUTLSConnection(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "example.com"}, utls.HelloChrome_Auto)
	w := &utlsConnWrap{UConn: uc}
	if _, err := vless.NewVisionConn(w, w, [16]byte{}, newNopLogger()); err != nil {
		t.Fatalf("vision refuses ordinary uTLS: %v", err)
	}
}

func TestVisionTLSRoundTripWithFingerprints(t *testing.T) {
	cert := selfSigned(t)
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	svc := vlessService(vless.FlowVision)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				if err := svc.NewConnection(context.Background(), c, M.SocksaddrFromNet(c.RemoteAddr()), nil); err != nil {
					_ = c.Close()
				}
			}(c)
		}
	}()
	for _, name := range []string{"chrome", "qq", "firefox", "random", "randomized", "none", "pinned-firefox", "fragment-firefox", "pinned-fragment-firefox"} {
		t.Run(name, func(t *testing.T) {
			fp := name
			if strings.Contains(name, "firefox") {
				fp = "firefox"
			}
			q := url.Values{"security": {"tls"}, "sni": {"localhost"}, "fp": {fp}, "flow": {"xtls-rprx-vision"}}
			pinned := strings.Contains(name, "pinned")
			if pinned {
				pin := sha256.Sum256(cert.Certificate[0])
				q.Set("pcs", fmt.Sprintf("%X", pin))
				q.Set("sni", "")
			}
			if strings.Contains(name, "fragment") {
				q.Set("fm", helloFragment)
			}
			h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@%s?%s", vlessTestUUID, ln.Addr(), q.Encode())).(*vlessHandler)
			switch cfg := h.tls.(type) {
			case *utlsConfig:
				if !pinned {
					cfg.cfg.RootCAs = roots
				}
			case *stdTLSConfig:
				if !pinned {
					cfg.cfg.RootCAs = roots
				}
			default:
				t.Fatalf("unexpected TLS type %T", cfg)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c, err := h.DialTCP(ctx, "tcp", "203.0.113.5:80")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(3 * time.Second))
			msg := []byte("GET / HTTP/1.0\r\n\r\n")
			if _, err := c.Write(msg); err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(msg))
			if _, err := io.ReadFull(c, got); err != nil || string(got) != string(msg) {
				t.Fatalf("echo=%q error=%v", got, err)
			}
			echoUDP(t, h)
		})
	}
}
