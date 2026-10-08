package outbound

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-vmess/vless"
	M "github.com/sagernet/sing/common/metadata"
	"golang.org/x/net/http2"
	"golang.org/x/net/websocket"
)

// seen records what the transport sent in its HTTP request.
type seen struct {
	mu                                   sync.Mutex
	host, path, query, header, ctype, ua string
}

func (s *seen) set(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.host, s.path, s.query = r.Host, r.URL.Path, r.URL.RawQuery
	s.header, s.ctype, s.ua = r.Header.Get("X-Test"), r.Header.Get("Content-Type"), r.Header.Get("User-Agent")
}

func vlessService(flow string) *vless.Service[int] {
	svc := vless.NewService[int](newNopLogger(), echoServer{})
	svc.UpdateUsers([]int{0}, []string{vlessTestUUID}, []string{flow})
	return svc
}

// serve runs the VLESS service on c and returns when the stream is finished.
func serve(svc *vless.Service[int], c net.Conn) {
	done := make(chan struct{})
	var once sync.Once
	finish := func(error) { once.Do(func() { close(done) }) }
	if err := svc.NewConnection(context.Background(), c, M.ParseSocksaddr("127.0.0.1:50000"), finish); err != nil {
		return
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
	}
}

func tlsListener(t *testing.T, protos ...string) net.Listener {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}, NextProtos: protos})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// wsServer is VLESS behind a WebSocket from golang.org/x/net, an
// implementation that is not ours.
func wsServer(t *testing.T, path string, rec *seen) int {
	t.Helper()
	svc := vlessService("")
	mux := http.NewServeMux()
	mux.Handle(path, websocket.Server{
		Handshake: func(c *websocket.Config, r *http.Request) error { rec.set(r); return nil },
		Handler: func(ws *websocket.Conn) {
			ws.PayloadType = websocket.BinaryFrame
			serve(svc, ws)
		},
	})
	ln := tlsListener(t, "http/1.1")
	srv := &http.Server{Handler: mux}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().(*net.TCPAddr).Port
}

// gunConn is the server side of the grpc transport on top of an HTTP/2 stream,
// written here from the protocol description so that it does not share code
// with the client.
type gunConn struct {
	body io.Reader
	w    http.ResponseWriter
	f    http.Flusher
	r    *http.Request
	left []byte
	wmu  sync.Mutex
	done bool // the handler has returned, the ResponseWriter is gone
}

// finish marks the stream as over; later writes fail instead of panicking.
func (g *gunConn) finish() {
	g.wmu.Lock()
	g.done = true
	g.wmu.Unlock()
}

func (g *gunConn) Read(p []byte) (int, error) {
	for len(g.left) == 0 {
		var hdr [5]byte
		if _, err := io.ReadFull(g.body, hdr[:]); err != nil {
			return 0, err
		}
		msg := make([]byte, binary.BigEndian.Uint32(hdr[1:]))
		if _, err := io.ReadFull(g.body, msg); err != nil {
			return 0, err
		}
		if len(msg) == 0 || msg[0] != 0x0a {
			return 0, fmt.Errorf("unexpected hunk %x", msg)
		}
		l, n := binary.Uvarint(msg[1:])
		g.left = msg[1+n : 1+n+int(l)]
	}
	n := copy(p, g.left)
	g.left = g.left[n:]
	return n, nil
}

func (g *gunConn) Write(p []byte) (int, error) {
	g.wmu.Lock()
	defer g.wmu.Unlock()
	if g.done {
		return 0, net.ErrClosed
	}
	var vl [binary.MaxVarintLen64]byte
	vn := binary.PutUvarint(vl[:], uint64(len(p)))
	msg := append(append([]byte{0x0a}, vl[:vn]...), p...)
	frame := append([]byte{0, 0, 0, 0, 0}, msg...)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(msg)))
	if _, err := g.w.Write(frame); err != nil {
		return 0, err
	}
	g.f.Flush()
	return len(p), nil
}

func (g *gunConn) Close() error                       { return nil }
func (g *gunConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (g *gunConn) RemoteAddr() net.Addr               { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (g *gunConn) SetDeadline(t time.Time) error      { return nil }
func (g *gunConn) SetReadDeadline(t time.Time) error  { return nil }
func (g *gunConn) SetWriteDeadline(t time.Time) error { return nil }

// grpcServer is VLESS behind a real HTTP/2 server at /<service>/Tun.
func grpcServer(t *testing.T, service string, rec *seen) int {
	t.Helper()
	svc := vlessService("")
	mux := http.NewServeMux()
	mux.HandleFunc("/"+service+"/Tun", func(w http.ResponseWriter, r *http.Request) {
		rec.set(r)
		w.Header().Set("Content-Type", "application/grpc")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		g := &gunConn{body: r.Body, w: w, f: f, r: r}
		defer g.finish()
		serve(svc, g)
	})
	srv := &http.Server{Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}}}
	if err := http2.ConfigureServer(srv, &http2.Server{}); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	return ln.Addr().(*net.TCPAddr).Port
}

func echoTCP(t *testing.T, h Handler) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := h.DialTCP(ctx, "tcp", "203.0.113.5:80")
	if err != nil {
		t.Fatalf("DialTCP: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	// more than one frame, so chunking is exercised
	msg := []byte(strings.Repeat("0123456789abcdef", 5000))
	go func() { _, _ = c.Write(msg) }()
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatal("the echo differs from what was sent")
	}
}

func echoUDP(t *testing.T, h Handler) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pc, err := h.DialUDP(ctx, "udp", "203.0.113.5:5353")
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer pc.Close()
	for i := 0; i < 3; i++ {
		msg := []byte(fmt.Sprintf("udp-%d", i))
		if _, err := pc.WriteTo(msg, nil); err != nil {
			t.Fatal(err)
		}
		_ = pc.SetReadDeadline(time.Now().Add(10 * time.Second))
		got := make([]byte, 1500)
		n, from, err := pc.ReadFrom(got)
		if err != nil || string(got[:n]) != string(msg) || from.String() != "203.0.113.5:5353" {
			t.Fatalf("got %q from %v err %v", got[:n], from, err)
		}
	}
}

func TestVLESSOverWebSocket(t *testing.T) {
	rec := &seen{}
	port := wsServer(t, "/vl", rec)
	link := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=ws&path=%%2Fvl%%3Fed%%3D2048&host=cdn.example.com", vlessTestUUID, port)
	h := newTestVLESSLink(t, link)
	echoTCP(t, h)
	echoUDP(t, h)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.path != "/vl" || rec.query != "ed=2048" {
		t.Fatalf("request went to %q ? %q", rec.path, rec.query)
	}
	if rec.host != "cdn.example.com" {
		t.Fatalf("Host header %q, want the host from the link", rec.host)
	}
}

func TestVLESSOverGRPC(t *testing.T) {
	rec := &seen{}
	port := grpcServer(t, "myservice", rec)
	link := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=grpc&serviceName=myservice", vlessTestUUID, port)
	h := newTestVLESSLink(t, link)
	echoTCP(t, h)
	echoUDP(t, h)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.path != "/myservice/Tun" || !strings.HasPrefix(rec.ctype, "application/grpc") {
		t.Fatalf("request: path %q content-type %q", rec.path, rec.ctype)
	}
	if rec.host != "localhost" {
		t.Fatalf("authority %q, want the server name", rec.host)
	}
}

func TestVLESSTransportALPN(t *testing.T) {
	cases := map[string][]string{
		"type=tcp":                              {"h2", "http/1.1"},
		"type=ws&path=/x":                       {"http/1.1"},
		"type=ws&path=/x&alpn=h2":               {"h2"},
		"type=grpc&serviceName=s":               {"h2"},
		"type=grpc&serviceName=s&alpn=http/1.1": {"h2"},
	}
	for q, want := range cases {
		link := "vless://" + vlessTestUUID + "@example.com:443?security=tls&sni=example.com&" + q
		h := newTestVLESSLink(t, link).(*vlessHandler)
		got := h.tls.NextProtos()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: ALPN %v, want %v", q, got, want)
		}
	}
}

func TestVLESSLinkOptionsReachTLS(t *testing.T) {
	h := newTestVLESSLink(t, "vless://"+vlessTestUUID+"@example.com:443?security=tls&sni=front.example.com&allowInsecure=1&alpn=h2,http/1.1").(*vlessHandler)
	cfg, err := h.tls.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify || cfg.ServerName != "front.example.com" {
		t.Fatalf("insecure=%v server name %q", cfg.InsecureSkipVerify, cfg.ServerName)
	}
	h = newTestVLESSLink(t, "vless://"+vlessTestUUID+"@example.com:443?security=tls&sni=example.com").(*vlessHandler)
	if cfg, _ := h.tls.Config(); cfg.InsecureSkipVerify {
		t.Fatal("certificate checks are off without being asked")
	}
}

// A link this engine cannot carry must not stop the engine: the handler is
// built and the error shows when the channel is used.
func TestVLESSUnsupportedLinkFailsOnUseNotOnCreation(t *testing.T) {
	h := newTestVLESSLink(t, "vless://"+vlessTestUUID+"@example.com:443?security=tls&sni=example.com&type=ws&path=/x&flow=xtls-rprx-vision")
	_, err := h.DialTCP(context.Background(), "tcp", "203.0.113.5:80")
	if err == nil || !strings.Contains(err.Error(), "xtls-rprx-vision") {
		t.Fatalf("got %v", err)
	}
	if _, err := h.DialUDP(context.Background(), "udp", "203.0.113.5:53"); err == nil {
		t.Fatal("UDP dialled over a link that cannot work")
	}
	if _, err := vlessTransportFrom(map[string]any{"transport": map[string]any{"type": "splithttp"}}, "", "x"); err == nil || !strings.Contains(err.Error(), "splithttp") {
		t.Fatalf("unknown transport: %v", err)
	}
}
