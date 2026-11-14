package netfetch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	SubscriptionProxyPort     = 1611
	SubscriptionProxyURL      = "http://127.0.0.1:1611"
	SubscriptionChannelHeader = "X-Hybrid-Failover-Channel"
)

// SubscriptionClients discovers channels in the running engine. No subscription
// URL or credentials are sent during discovery. DNS for HTTPS destinations is
// resolved by the tunnel, not by the ISP resolver on the CLI process.
func SubscriptionClients(ctx context.Context) []*http.Client {
	return subscriptionClients(ctx, SubscriptionProxyURL)
}

func subscriptionClients(ctx context.Context, proxyURL string) []*http.Client {
	discovery := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer discovery.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL+"/channels", nil)
	if err != nil {
		return nil
	}
	resp, err := discovery.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var tags []string
	if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&tags) != nil {
		return nil
	}
	proxy, _ := url.Parse(proxyURL)
	clients := make([]*http.Client, 0, len(tags))
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		transport := &http.Transport{
			Proxy:              http.ProxyURL(proxy),
			ProxyConnectHeader: http.Header{SubscriptionChannelHeader: []string{tag}},
			DisableKeepAlives:  true,
		}
		clients = append(clients, &http.Client{Timeout: 15 * time.Second, Transport: channelTransport{transport, tag}})
	}
	return clients
}

type channelTransport struct {
	transport *http.Transport
	tag       string
}

func (t channelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if clone.URL.Scheme == "http" {
		clone.Header.Set(SubscriptionChannelHeader, t.tag)
	}
	return t.transport.RoundTrip(clone)
}
func (t channelTransport) CloseIdleConnections() { t.transport.CloseIdleConnections() }
