package netfetch

import (
	"net/http"
	"testing"
)

func TestHTTPClientUsesExplicitDNS(t *testing.T) {
	client := HTTPClient(DefaultDNSAddr, "", defaultTimeout)
	if client == nil {
		t.Fatal("expected client")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.DialContext == nil {
		t.Fatal("expected transport with DialContext")
	}
}

func TestHTTPClientWithProxyURL(t *testing.T) {
	client := HTTPClient(DefaultDNSAddr, "http://127.0.0.1:1610", defaultTimeout)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("expected proxy on transport")
	}
}
