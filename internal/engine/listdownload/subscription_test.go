package listdownload

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
)

func TestSubscriptionProxyHTTPAndHTTPS(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "http", true: "https"}[secure], func(t *testing.T) {
			panel := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !secure && !r.Close {
					t.Error("HTTP upstream must close after one response")
				}
				if r.Header.Get(netfetch.SubscriptionChannelHeader) != "" {
					t.Error("internal channel header leaked to panel")
				}
				_, _ = io.WriteString(w, "subscription")
			}))
			if secure {
				panel.StartTLS()
			} else {
				panel.Start()
			}
			defer panel.Close()
			reg, err := outbound.NewRegistry(context.Background(), []plan.OutboundPlan{{Tag: "test-channel", Kind: plan.OutboundDirect}})
			if err != nil {
				t.Fatal(err)
			}
			defer reg.Stop()
			s := NewSubscription([]plan.ChannelPlan{{Tag: "test-channel"}}, reg)
			proxyURL, _ := url.Parse("http://local-proxy")
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), ProxyConnectHeader: http.Header{netfetch.SubscriptionChannelHeader: []string{"test-channel"}}, DisableKeepAlives: true,
				DialContext: func(context.Context, string, string) (net.Conn, error) {
					client, server := net.Pipe()
					go s.handleConn(server)
					return client, nil
				},
			}
			if secure {
				roots := x509.NewCertPool()
				roots.AddCert(panel.Certificate())
				transport.TLSClientConfig = &tls.Config{RootCAs: roots}
			}
			defer transport.CloseIdleConnections()
			req, _ := http.NewRequest("GET", panel.URL, nil)
			if !secure {
				req.Header.Set(netfetch.SubscriptionChannelHeader, "test-channel")
			}
			resp, err := (&http.Client{Transport: transport, Timeout: 3 * time.Second}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil || string(body) != "subscription" {
				t.Fatalf("body=%q err=%v", body, err)
			}
		})
	}
}

func TestSubscriptionProxyDiscoveryAndRejectsUnknownChannel(t *testing.T) {
	s := NewSubscription([]plan.ChannelPlan{{Tag: "one"}, {Tag: "one"}, {Tag: "two"}}, nil)
	for _, tc := range []struct {
		request string
		status  int
		body    string
	}{
		{"GET /channels HTTP/1.1\r\nHost: localhost\r\n\r\n", 200, `["one","two"]`},
		{"CONNECT panel.example:443 HTTP/1.1\r\nHost: panel.example:443\r\nX-Hybrid-Failover-Channel: unknown\r\n\r\n", 403, ""},
		{"CONNECT panel.example:443 HTTP/1.1\r\nHost: panel.example:443\r\n\r\n", 403, ""},
	} {
		c, server := net.Pipe()
		go s.handleConn(server)
		_, _ = io.WriteString(c, tc.request)
		resp, err := http.ReadResponse(bufio.NewReader(c), nil)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.Close()
		if resp.StatusCode != tc.status || strings.TrimSpace(string(body)) != tc.body {
			t.Fatalf("status=%d body=%q", resp.StatusCode, body)
		}
	}
}

func TestSubscriptionStopClosesBothHalves(t *testing.T) {
	s := NewSubscription(nil, nil)
	s.port = 0 // reserve an ephemeral loopback port
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	client, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// A partial header keeps the accepted connection open until Stop.
	_, _ = io.WriteString(client, "GET /channels HTTP/1.1\r\n")
	remote, panel := net.Pipe()
	defer remote.Close()
	defer panel.Close()
	if !s.trackRemote(remote) {
		t.Fatal("remote not registered")
	}
	s.Stop()
	for _, conn := range []net.Conn{client, panel} {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		_, err := conn.Read(make([]byte, 1))
		if err == nil {
			t.Fatal("connection survived Stop")
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("Stop did not close connection")
		}
	}
	if s.trackRemote(remote) {
		t.Fatal("registered a remote after Stop")
	}
}

func TestConnectForwardsAlreadyBufferedPayload(t *testing.T) {
	panel, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer panel.Close()
	echoed := make(chan error, 1)
	go func() {
		conn, err := panel.Accept()
		if err != nil {
			echoed <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		data := make([]byte, 4)
		_, err = io.ReadFull(conn, data)
		if err == nil {
			_, err = conn.Write(data)
		}
		echoed <- err
	}()
	reg, err := outbound.NewRegistry(context.Background(), []plan.OutboundPlan{{Tag: "test", Kind: plan.OutboundDirect}})
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()
	s := NewSubscription([]plan.ChannelPlan{{Tag: "test"}}, reg)
	client, server := net.Pipe()
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	done := make(chan struct{})
	go func() { s.handleConn(server); close(done) }()
	// Sending the first tunneled bytes together with CONNECT makes ReadRequest
	// prefetch them. Reading all of br would wait for EOF and deadlock TLS.
	_, err = io.WriteString(client, "CONNECT "+panel.Addr().String()+" HTTP/1.1\r\nHost: "+panel.Addr().String()+"\r\nX-Hybrid-Failover-Channel: test\r\n\r\nping")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(client)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("response=%v err=%v", resp, err)
	}
	payload := make([]byte, 4)
	if _, err = io.ReadFull(reader, payload); err != nil || string(payload) != "ping" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	client.Close()
	if err := <-echoed; err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("CONNECT handler survived closed endpoints")
	}
}
