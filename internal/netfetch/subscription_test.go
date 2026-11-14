package netfetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscriptionClientsSelectDiscoveredChannels(t *testing.T) {
	discoveries, downloads := 0, 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() && r.URL.Path == "/channels" {
			discoveries++
			_, _ = io.WriteString(w, `["tunnel-one","tunnel-two"]`)
			return
		}
		downloads++
		if r.URL.String() != "http://blocked.example/private-token" {
			t.Errorf("unexpected proxy destination %q", r.URL)
		}
		expected := []string{"tunnel-one", "tunnel-two"}[downloads-1]
		if r.Header.Get(SubscriptionChannelHeader) != expected {
			t.Errorf("channel = %q", r.Header.Get(SubscriptionChannelHeader))
		}
		_, _ = io.WriteString(w, "subscription")
	}))
	defer proxy.Close()
	clients := subscriptionClients(context.Background(), proxy.URL)
	if len(clients) != 2 {
		t.Fatalf("clients=%d", len(clients))
	}
	for _, client := range clients {
		resp, err := client.Get("http://blocked.example/private-token")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		client.CloseIdleConnections()
	}
	if discoveries != 1 || downloads != 2 {
		t.Fatalf("discovery=%d downloads=%d", discoveries, downloads)
	}
}

func TestSubscriptionClientsWithoutEngine(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if got := subscriptionClients(context.Background(), server.URL); len(got) != 0 {
		t.Fatalf("clients=%d", len(got))
	}
}
