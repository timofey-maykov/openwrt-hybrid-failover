package outbound

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

type xhttpTestRoundTripper func(*http.Request) (*http.Response, error)

func (f xhttpTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func blockedXHTTP(t *testing.T) net.Conn {
	t.Helper()
	rt := xhttpTestRoundTripper(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	c, err := newXHTTPStream(context.Background(), rt, xhttpCloseFunc(func() error { return nil }), &net.TCPAddr{}, &net.TCPAddr{}, "https", "example.com", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func TestXHTTPDeadlines(t *testing.T) {
	for _, op := range []string{"read", "write"} {
		t.Run(op, func(t *testing.T) {
			c := blockedXHTTP(t)
			deadline := time.Now().Add(25 * time.Millisecond)
			done := make(chan error, 1)
			if op == "read" {
				_ = c.SetReadDeadline(deadline)
				go func() { _, err := c.Read(make([]byte, 1)); done <- err }()
			} else {
				_ = c.SetWriteDeadline(deadline)
				go func() { _, err := c.Write([]byte{1}); done <- err }()
			}
			select {
			case err := <-done:
				if !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatalf("%s error=%v", op, err)
				}
				var timeout net.Error
				if !errors.As(err, &timeout) || !timeout.Timeout() {
					t.Fatalf("not a timeout: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("deadline did not unblock I/O")
			}
		})
	}
}
func TestXHTTPClearedDeadline(t *testing.T) {
	c := blockedXHTTP(t)
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	_ = c.SetReadDeadline(time.Time{})
	timer := time.AfterFunc(40*time.Millisecond, func() { _ = c.Close() })
	defer timer.Stop()
	_, err := c.Read(make([]byte, 1))
	if errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("cleared deadline fired")
	}
}
