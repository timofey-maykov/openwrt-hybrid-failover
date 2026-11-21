package outbound

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/http2"
)

type xhttpTestSession struct {
	r       *io.PipeReader
	w       *io.PipeWriter
	mu      sync.Mutex
	next    uint64
	pending map[uint64][]byte
}

// An independent server implementation exercises GET/POST session pairing
// and numbered, potentially reordered packet uploads, including HTTP/1.1
// with full duplex explicitly enabled by its server.
func xhttpModeServer(t *testing.T, proto string) int {
	t.Helper()
	var mu sync.Mutex
	sessions := map[string]*xhttpTestSession{}
	svc := vlessService("")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if proto == "http/1.1" {
			_ = http.NewResponseController(w).EnableFullDuplex()
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/x/"), "/")
		if len(parts) == 1 && parts[0] == "" {
			w.WriteHeader(200)
			f := w.(http.Flusher)
			f.Flush()
			serve(svc, &rawHTTPBodyConn{body: r.Body, w: w, f: f})
			return
		}
		session := parts[0]
		mu.Lock()
		s := sessions[session]
		if s == nil {
			pr, pw := io.Pipe()
			s = &xhttpTestSession{r: pr, w: pw, pending: map[uint64][]byte{}}
			sessions[session] = s
		}
		mu.Unlock()
		if r.Method == http.MethodGet {
			stop := context.AfterFunc(r.Context(), func() { _ = s.r.Close(); _ = s.w.Close() })
			defer stop()
			defer s.r.Close()
			defer s.w.Close()
			w.WriteHeader(200)
			f := w.(http.Flusher)
			f.Flush()
			serve(svc, &rawHTTPBodyConn{body: s.r, w: w, f: f})
			return
		}
		if len(parts) == 1 {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			_, _ = io.Copy(s.w, r.Body)
			_ = s.w.Close()
			return
		}
		seq, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			http.Error(w, "sequence", 400)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "body", 400)
			return
		}
		s.mu.Lock()
		s.pending[seq] = body
		for {
			b, ok := s.pending[s.next]
			if !ok {
				break
			}
			delete(s.pending, s.next)
			s.next++
			_, err = s.w.Write(b)
			if err != nil {
				break
			}
		}
		s.mu.Unlock()
		if err != nil {
			http.Error(w, "upload", 500)
			return
		}
		w.WriteHeader(200)
	})
	cert := selfSigned(t)
	if proto == "h3" {
		port := xhttp3Server(t, cert, handler)
		t.Cleanup(func() {
			mu.Lock()
			defer mu.Unlock()
			for _, s := range sessions {
				_ = s.r.Close()
				_ = s.w.Close()
			}
		})
		return port
	}
	srv := &http.Server{Handler: handler, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{proto}}}
	if proto == "h2" {
		if err := http2.ConfigureServer(srv, &http2.Server{}); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, s := range sessions {
			_ = s.w.Close()
			_ = s.r.Close()
		}
	})
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestXHTTPTransportModes(t *testing.T) {
	for _, proto := range []string{"http/1.1", "h2", "h3"} {
		port := xhttpModeServer(t, proto)
		for _, mode := range []string{"auto", "stream-one", "stream-up", "packet-up"} {
			t.Run(proto+"/"+mode, func(t *testing.T) {
				link := fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&alpn=%s&mode=%s&path=%%2Fx", vlessTestUUID, port, proto, mode)
				h := newTestVLESSLink(t, link)
				echoTCP(t, h)
				echoUDP(t, h)
			})
		}
	}
}

func TestXHTTPPacketWriterPreservesInput(t *testing.T) {
	// POST buffering must own its bytes after Write returns. Callers reuse
	// their buffers immediately, especially in tproxy's packet paths.
	port := xhttpModeServer(t, "h3")
	h := newTestVLESSLink(t, fmt.Sprintf("vless://%s@127.0.0.1:%d?security=tls&sni=localhost&allowInsecure=1&type=xhttp&alpn=h3&mode=packet-up&path=%%2Fx", vlessTestUUID, port))
	c, err := h.DialTCP(context.Background(), "tcp", "203.0.113.5:80")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	want := bytes.Repeat([]byte{0x53}, 192<<10)
	sent := append([]byte(nil), want...)
	if _, err := c.Write(sent); err != nil {
		t.Fatal(err)
	}
	clear(sent)
	got := make([]byte, len(want))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("caller buffer was retained by the upload queue")
	}
}
