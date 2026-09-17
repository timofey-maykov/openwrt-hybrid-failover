package outbound

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

type fakeProbeHandler struct {
	dialDelay time.Duration
	response  string
}

func (f *fakeProbeHandler) Tag() string  { return "fake" }
func (f *fakeProbeHandler) Close() error { return nil }
func (f *fakeProbeHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, net.ErrClosed
}
func (f *fakeProbeHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	c1, c2 := net.Pipe()
	go func() {
		time.Sleep(f.dialDelay)
		buf := make([]byte, 1024)
		_, _ = c2.Read(buf)
		_, _ = io.WriteString(c2, f.response)
		_ = c2.Close()
	}()
	return c1, nil
}

func TestProbeURLTestHTTPWaitsForResponse(t *testing.T) {
	h := &fakeProbeHandler{
		dialDelay: 30 * time.Millisecond,
		response:  "HTTP/1.1 204 No Content\r\n\r\n",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ms, err := probeURLTestHTTP(ctx, h, "http://www.gstatic.com/generate_204")
	if err != nil {
		t.Fatal(err)
	}
	if ms < 20 {
		t.Fatalf("expected probe to include response wait, got %dms", ms)
	}
}

func TestProbeRejectsUnusableResponse(t *testing.T) {
	for _, response := range []string{"HTTP/1.1 503 Service Unavailable\r\n\r\n", "not HTTP\r\n", "HTTP/1.1 302 Found\r\nLocation: /login\r\n\r\n"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := ProbeHTTP(ctx, &fakeProbeHandler{response: response}, "http://example.test/")
		cancel()
		if err == nil {
			t.Errorf("accepted %q", response)
		}
	}
}

func TestProbeHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ProbeHTTP(ctx, &fakeProbeHandler{dialDelay: 50 * time.Millisecond, response: "HTTP/1.1 204 No Content\r\n\r\n"}, "http://example.test/")
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled probe succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("probe ignored cancellation")
	}
}
