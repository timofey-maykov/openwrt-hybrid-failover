package subscription

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

type Fetcher struct {
	HTTP *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{HTTP: &http.Client{Timeout: 60 * time.Second}}
}

func (f *Fetcher) FetchURLs(urls []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, u := range urls {
		links, err := f.FetchOne(u)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if _, ok := seen[link]; ok {
				continue
			}
			if err := validation.ValidateProxyURI(link); err != nil {
				continue
			}
			seen[link] = struct{}{}
			out = append(out, link)
		}
	}
	return out, nil
}

func (f *Fetcher) FetchOne(subURL string) ([]string, error) {
	resp, err := f.HTTP.Get(strings.TrimSpace(subURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(body))
	if dec, err := base64.StdEncoding.DecodeString(text); err == nil {
		text = string(dec)
	}
	var links []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		links = append(links, line)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("subscription empty: %s", subURL)
	}
	return links, nil
}
