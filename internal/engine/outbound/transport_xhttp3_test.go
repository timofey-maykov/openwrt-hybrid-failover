package outbound

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/quic-go/http3"
)

func xhttp3Server(t *testing.T, cert tls.Certificate, handler http.Handler) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http3.Server{TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}, Handler: handler}
	t.Cleanup(func() { _ = srv.Close(); _ = pc.Close() })
	go func() { _ = srv.Serve(pc) }()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func TestVLESSOverXHTTP3(t *testing.T) {
	for _, mode := range []string{"stream-one"} {
		t.Run(mode, func(t *testing.T) {
			rec := &seen{}
			var requests atomic.Int32
			svc := vlessService("")
			port := xhttp3Server(t, selfSigned(t), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.ProtoMajor != 3 || r.TLS == nil || r.TLS.ServerName != "localhost" || r.Method != "POST" {
					t.Errorf("unexpected request: %s %s TLS=%v", r.Proto, r.Method, r.TLS)
					http.Error(w, "request", 400)
					return
				}
				rec.set(r)
				requests.Add(1)
				w.WriteHeader(200)
				f := w.(http.Flusher)
				f.Flush()
				serve(svc, &rawHTTPBodyConn{body: r.Body, w: w, f: f})
			}))
			// A different Host and an explicit server port must not change the UDP
			// destination or the SNI. A TCP uTLS fp must not force a TCP connection.
			link := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&fp=firefox&type=xhttp&path=%%2Fx%%3Ftoken%%3Dtest&mode=%s&host=cdn.example.com&alpn=h3", vlessTestUUID, port, mode)
			h := newTestVLESSLink(t, link)
			echoTCP(t, h)
			echoUDP(t, h)
			if requests.Load() != 2 {
				t.Fatalf("requests=%d", requests.Load())
			}
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if rec.host != "cdn.example.com" || rec.path != "/x/" || rec.query != "token=test" || rec.ctype != "application/grpc" {
				t.Fatalf("request: host=%q path=%q query=%q type=%q", rec.host, rec.path, rec.query, rec.ctype)
			}
		})
	}
}

func TestXHTTP3CertificateValidation(t *testing.T) {
	cert := selfSigned(t)
	port := xhttp3Server(t, cert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	pin := fmt.Sprintf("%x", sha256.Sum256(cert.Certificate[0]))
	for _, tc := range []struct {
		name, options string
		wantOK        bool
	}{
		{"untrusted", "", false},
		{"pin", "&pcs=" + pin, true},
		{"bad-pin-even-insecure", "&allowInsecure=1&pcs=" + strings.Repeat("0", 64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&type=xhttp&mode=stream-one&alpn=h3%s", vlessTestUUID, port, tc.options)).(*vlessHandler)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c, err := h.dialStream(ctx)
			if c != nil {
				_ = c.Close()
			}
			if (err == nil) != tc.wantOK {
				t.Fatalf("err=%v wantOK=%v", err, tc.wantOK)
			}
		})
	}
}

func TestXHTTP3UnsupportedModesAndVision(t *testing.T) {
	for _, options := range []string{"mode=invalid", "flow=xtls-rprx-vision"} {
		h := newTestVLESSLink(t, "vless://"+vlessTestUUID+"@127.0.0.1:1?security=tls&type=xhttp&alpn=h3&"+options).(*vlessHandler)
		if _, err := h.dialStream(context.Background()); err == nil || strings.Contains(err.Error(), "connection refused") {
			t.Fatalf("%s: %v", options, err)
		}
	}
}

func TestXHTTP3CloseUnblocksRead(t *testing.T) {
	port := xhttp3Server(t, selfSigned(t), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&alpn=h3", vlessTestUUID, port)).(*vlessHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, err := h.dialStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := c.Read(make([]byte, 1)); done <- err }()
	_ = c.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("read succeeded after close")
		}
	case <-time.After(time.Second):
		t.Fatal("read stuck after close")
	}
}

func TestXHTTP3UDPOutlivesDialContext(t *testing.T) {
	svc := vlessService("")
	port := xhttp3Server(t, selfSigned(t), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		f := w.(http.Flusher)
		f.Flush()
		serve(svc, &rawHTTPBodyConn{body: r.Body, w: w, f: f})
	}))
	h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&mode=stream-one&alpn=h3", vlessTestUUID, port))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	c, err := h.DialUDP(ctx, "udp", "203.0.113.5:5353")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	done := make(chan error, 1)
	go func() {
		if _, err := c.WriteTo([]byte("after-cancel"), nil); err != nil {
			done <- err
			return
		}
		b := make([]byte, 64)
		n, _, err := c.ReadFrom(b)
		if err == nil && string(b[:n]) != "after-cancel" {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UDP stalled after dial context cancellation")
	}
}

func TestXHTTP3RejectsRealityAndTCPFragmentation(t *testing.T) {
	for _, tc := range []struct{ options, want string }{

		{"security=tls&fm=" + url.QueryEscape(`{"tcp":[{"type":"fragment","settings":{"length":"10-20","interval":"1-2","packets":"tlshello"}}]}`), "fragmentation"},
	} {
		h := newTestVLESSLink(t, "vless://"+vlessTestUUID+"@127.0.0.1:1?type=xhttp&alpn=h3&"+tc.options).(*vlessHandler)
		_, err := h.dialStream(context.Background())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("want %q, got %v", tc.want, err)
		}
	}
}

func TestXHTTPRealitySelectsH2(t *testing.T) {
	h := newTestVLESSLink(t, "vless://"+vlessTestUUID+"@127.0.0.1:1?type=xhttp&alpn=h3&security=reality&pbk="+strings.Repeat("A", 43)).(*vlessHandler)
	if h.initErr != nil || h.transport.http3 || h.transport.http1 || h.transport.mode != "stream-one" || strings.Join(h.tls.NextProtos(), ",") != "h2" {
		t.Fatalf("REALITY transport: %+v error=%v", h.transport, h.initErr)
	}
}
