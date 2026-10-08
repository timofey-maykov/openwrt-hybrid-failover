package outbound

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-vmess/vless"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

const vlessTestUUID = "b831381d-6324-4d53-ad4f-8cda48b30811"

// echoServer answers every TCP stream and every UDP packet with what it got.
type echoServer struct{}

func (echoServer) NewConnectionEx(ctx context.Context, conn net.Conn, source, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()
}

func (echoServer) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	go func() {
		defer conn.Close()
		for {
			// room in front of the payload for the protocol header
			headroom := N.CalculateFrontHeadroom(conn)
			buffer := buf.NewSize(headroom + 2048)
			buffer.Resize(headroom, 0)
			dest, err := conn.ReadPacket(buffer)
			if err != nil {
				buffer.Release()
				return
			}
			if err := conn.WritePacket(buffer, dest); err != nil {
				return
			}
		}
	}()
}

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// vlessServer is a real VLESS server (the library's own) behind TLS on loopback.
func vlessServer(t *testing.T, flow string) int {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	svc := vless.NewService[int](newNopLogger(), echoServer{})
	svc.UpdateUsers([]int{0}, []string{vlessTestUUID}, []string{flow})
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
	return ln.Addr().(*net.TCPAddr).Port
}

func newTestVLESS(t *testing.T, port int, flow string) Handler {
	t.Helper()
	uri := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&type=tcp", vlessTestUUID, port)
	if flow != "" {
		uri += "&flow=" + flow
	}
	h, err := newVLESSHandler(plan.OutboundPlan{Tag: "vless-test", ProxyURI: uri})
	if err != nil {
		t.Fatal(err)
	}
	// the server certificate is self-signed
	h.(*vlessHandler).tls = newSTDConfig(tlsOptions{enabled: true, serverName: "localhost", insecure: true})
	return h
}

func TestVLESSCarriesUDP(t *testing.T) {
	for _, flow := range []string{"", "xtls-rprx-vision"} {
		flow := flow
		t.Run("flow="+flow, func(t *testing.T) {
			h := newTestVLESS(t, vlessServer(t, flow), flow)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			pc, err := h.DialUDP(ctx, "udp", "203.0.113.5:5353")
			if err != nil {
				t.Fatalf("DialUDP: %v", err)
			}
			defer pc.Close()
			for i := 0; i < 3; i++ {
				msg := []byte(fmt.Sprintf("packet-%d", i))
				if _, err := pc.WriteTo(msg, nil); err != nil {
					t.Fatalf("WriteTo: %v", err)
				}
				_ = pc.SetReadDeadline(time.Now().Add(5 * time.Second))
				got := make([]byte, 1500)
				n, from, err := pc.ReadFrom(got)
				if err != nil {
					t.Fatalf("ReadFrom: %v", err)
				}
				if string(got[:n]) != string(msg) {
					t.Fatalf("got %q, want %q", got[:n], msg)
				}
				if from.String() != "203.0.113.5:5353" {
					t.Fatalf("reply came from %v", from)
				}
			}
		})
	}
}

func TestVLESSStillCarriesTCP(t *testing.T) {
	h := newTestVLESS(t, vlessServer(t, ""), "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := h.DialTCP(ctx, "tcp", "203.0.113.5:80")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(c, got); err != nil || string(got) != "hello" {
		t.Fatalf("%q %v", got, err)
	}
}
