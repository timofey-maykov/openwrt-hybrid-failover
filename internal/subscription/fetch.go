package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/secrets"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

type Fetcher struct {
	HTTP *http.Client
	// FallbackClients is lazy: a successful direct download never contacts the engine.
	FallbackClients func(context.Context) []*http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{HTTP: &http.Client{Timeout: 15 * time.Second}, FallbackClients: netfetch.SubscriptionClients}
}

// Link is one server from a subscription. Name is what the panel called it
// (the #fragment of the entry), empty when it gave none.
type Link struct {
	URI  string
	Name string
}

// Source describes one subscription URL after a fetch: what the panel said
// about itself in the response headers and how the fetch went. The URL is
// masked, so the struct can be stored and shown.
type Source struct {
	URL        string `json:"url"`
	Title      string `json:"title,omitempty"`
	SupportURL string `json:"support_url,omitempty"`
	// Expire is the Unix time the panel reports in Subscription-Userinfo, 0 when none.
	Expire  int64  `json:"expire,omitempty"`
	Servers int    `json:"servers"`
	Error   string `json:"error,omitempty"`
}

// Result is what a refresh learned from every configured URL.
type Result struct {
	Links   []Link
	Sources []Source
}

// URIs are the links alone, in order.
func (r Result) URIs() []string {
	out := make([]string, 0, len(r.Links))
	for _, l := range r.Links {
		out = append(out, l.URI)
	}
	return out
}

// FetchURLs returns the usable links of every URL. It fails on the first URL
// that cannot be fetched, like a cron job should.
func (f *Fetcher) FetchURLs(urls []string) ([]string, error) {
	res, err := f.Fetch(urls)
	if err != nil {
		return nil, err
	}
	return res.URIs(), nil
}

// Fetch reads every URL. A URL that fails ends the fetch with an error, but
// the Sources collected so far (the failed one included, with its Error set)
// are returned with it, so the caller can record what happened.
func (f *Fetcher) Fetch(urls []string) (Result, error) {
	return f.FetchContext(context.Background(), urls)
}

// FetchContext bounds the entire refresh, not each URL separately: callers such
// as the bot must have time left to apply the result before their RPC expires.
func (f *Fetcher) FetchContext(ctx context.Context, urls []string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	seen := map[string]struct{}{}
	var res Result
	for _, u := range urls {
		src, entries, err := f.fetchOne(ctx, u)
		if err != nil {
			src.Error = err.Error()
			res.Sources = append(res.Sources, src)
			return res, err
		}
		for _, raw := range entries {
			link, ok := usableLink(raw)
			if !ok {
				continue
			}
			if _, dup := seen[link.URI]; dup {
				continue
			}
			seen[link.URI] = struct{}{}
			res.Links = append(res.Links, link)
			src.Servers++
		}
		res.Sources = append(res.Sources, src)
	}
	if len(res.Links) == 0 {
		return res, fmt.Errorf("subscription has no supported links")
	}
	return res, nil
}

// usableLink converts an entry to what the router can use and keeps the
// panel's name for it.
func usableLink(raw string) (Link, bool) {
	name := entryName(raw)
	if amnezia.IsConfURI(raw) {
		conv, err := amnezia.ConfURIToAWG2(raw)
		if err != nil {
			return Link{}, false
		}
		raw = conv
	}
	if err := validation.ValidateProxyURI(raw); err != nil {
		return Link{}, false
	}
	return Link{URI: raw, Name: name}, true
}

// entryName is the #fragment of a subscription entry, which panels use for
// the server name, decoded and trimmed to something a label can hold.
func entryName(raw string) string {
	i := strings.IndexByte(raw, '#')
	if i < 0 {
		return ""
	}
	name := raw[i+1:]
	if dec, err := url.PathUnescape(name); err == nil {
		name = dec
	}
	name = strings.Join(strings.Fields(name), " ")
	if r := []rune(name); len(r) > 48 {
		name = string(r[:48])
	}
	return name
}

// FetchOne returns the raw entries of one subscription.
func (f *Fetcher) FetchOne(subURL string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	_, entries, err := f.fetchOne(ctx, subURL)
	return entries, err
}

func (f *Fetcher) fetchOne(ctx context.Context, subURL string) (Source, []string, error) {
	if err := ctx.Err(); err != nil {
		return Source{URL: secrets.MaskURL(subURL)}, nil, err
	}
	src, entries, err := fetchWithClient(ctx, f.HTTP, subURL)
	if err == nil && supportedEntries(entries) {
		return src, entries, nil
	}
	if err == nil {
		err = fmt.Errorf("subscription has no supported links: %s", src.URL)
	}
	if f.FallbackClients == nil || ctx.Err() != nil {
		return src, nil, err
	}
	clients := f.FallbackClients(ctx)
	defer func() {
		for _, client := range clients {
			client.CloseIdleConnections()
		}
	}()
	for _, client := range clients {
		if ctx.Err() != nil {
			break
		}
		next, links, retryErr := fetchWithClient(ctx, client, subURL)
		if retryErr == nil && supportedEntries(links) {
			return next, links, nil
		}
	}
	// Keep the panel's original reason (for example an expired subscription),
	// rather than replacing it with the last failed tunnel's transport error.
	return src, nil, err
}

func supportedEntries(entries []string) bool {
	for _, entry := range entries {
		if _, ok := usableLink(entry); ok {
			return true
		}
	}
	return false
}

func fetchWithClient(ctx context.Context, client *http.Client, subURL string) (Source, []string, error) {
	src := Source{URL: secrets.MaskURL(subURL)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(subURL), nil)
	if err != nil {
		return src, nil, errors.New(secrets.MaskSecrets(err.Error()))
	}
	resp, err := client.Do(req)
	if err != nil {
		// net/http puts the whole URL, token included, into its errors.
		return src, nil, errors.New(secrets.MaskSecrets(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return src, nil, errors.New(secrets.MaskSecrets(err.Error()))
	}
	if len(body) > 4<<20 {
		return src, nil, fmt.Errorf("subscription exceeds 4 MiB: %s", src.URL)
	}
	if resp.StatusCode/100 != 2 {
		return src, nil, fmt.Errorf("%s: %s", src.URL, httpStatusText(resp.StatusCode, body))
	}
	src.Title = strings.TrimSpace(resp.Header.Get("Profile-Title"))
	src.SupportURL = strings.TrimSpace(resp.Header.Get("Support-Url"))
	src.Expire = parseExpire(resp.Header.Get("Subscription-Userinfo"))

	text := strings.TrimSpace(string(body))
	if dec, err := base64.StdEncoding.DecodeString(text); err == nil {
		text = string(dec)
	}
	var entries []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	if len(entries) == 0 {
		return src, nil, fmt.Errorf("subscription empty: %s", src.URL)
	}
	return src, entries, nil
}

// httpStatusText names a failed status; a panel's plain-text body (first line)
// usually says why, for example that the subscription has expired.
func httpStatusText(code int, body []byte) string {
	msg := fmt.Sprintf("HTTP %d", code)
	first := strings.TrimSpace(strings.SplitN(string(body), "\n", 2)[0])
	if first != "" && !strings.HasPrefix(first, "<") && len(first) <= 120 {
		msg += ": " + first
	}
	return msg
}

// parseExpire reads expire=<unix> out of a Subscription-Userinfo header.
func parseExpire(h string) int64 {
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || strings.TrimSpace(k) != "expire" {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}
