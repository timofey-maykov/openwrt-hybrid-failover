package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

type ProxyInfo struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Now     string            `json:"now"`
	All     []string          `json:"all"`
	History []json.RawMessage `json:"history"`
}

type ProxiesResponse struct {
	Proxies map[string]ProxyInfo `json:"proxies"`
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Proxies(ctx context.Context) (ProxiesResponse, error) {
	var resp ProxiesResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/proxies", nil)
	if err != nil {
		return resp, err
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		return resp, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(r.Body)
		return resp, fmt.Errorf("clash api %s: %s", r.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) ActiveOutbound(ctx context.Context, selectorTag string) (string, error) {
	resp, err := c.Proxies(ctx)
	if err != nil {
		return "", err
	}
	p, ok := resp.Proxies[selectorTag]
	if !ok {
		return "", fmt.Errorf("selector %q not found", selectorTag)
	}
	return p.Now, nil
}

// LastDelayMS returns the most recent delay from Clash history, if any.
func (p ProxyInfo) LastDelayMS() int {
	for _, raw := range p.History {
		var item struct {
			Delay int `json:"delay"`
		}
		if json.Unmarshal(raw, &item) == nil && item.Delay > 0 {
			return item.Delay
		}
	}
	return 0
}

// ProxyDelay runs a live delay probe via Clash API.
func (c *Client) ProxyDelay(ctx context.Context, outbound, testURL string) (int, error) {
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	probeURL := c.BaseURL + "/proxies/" + url.PathEscape(outbound) +
		"/delay?timeout=5000&url=" + url.QueryEscape(testURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return 0, err
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		body, _ := io.ReadAll(r.Body)
		return 0, fmt.Errorf("probe %s: %s %s", outbound, r.Status, strings.TrimSpace(string(body)))
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if v, ok := payload["delay"]; ok {
		switch d := v.(type) {
		case float64:
			return int(d), nil
		case string:
			n, _ := strconv.Atoi(d)
			if n > 0 {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("no delay in response for %s", outbound)
}

// SwitchProxy sets the active outbound on a selector (Clash external controller API).
func (c *Client) SwitchProxy(ctx context.Context, selectorTag, outbound string) error {
	if selectorTag == "" || outbound == "" {
		return fmt.Errorf("selector and outbound are required")
	}
	values := url.Values{}
	values.Set("name", outbound)
	endpoint := c.BaseURL + "/proxies/" + url.PathEscape(selectorTag)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		body, _ := io.ReadAll(r.Body)
		return fmt.Errorf("switch %s -> %s: %s %s", selectorTag, outbound, r.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

