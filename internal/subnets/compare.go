package subnets

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
)

// NormalizeCIDRS returns sorted unique CIDR strings.
func NormalizeCIDRS(cidrs []string) []string {
	seen := make(map[string]struct{}, len(cidrs))
	out := make([]string, 0, len(cidrs))
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		out = append(out, cidr)
	}
	sort.Strings(out)
	return out
}

// CIDRSetsEqual reports whether two CIDR lists contain the same entries.
func CIDRSetsEqual(a, b []string) bool {
	na := NormalizeCIDRS(a)
	nb := NormalizeCIDRS(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// WriteListBodyIfChanged writes body to dest when parsed CIDRs differ from the local file.
func WriteListBodyIfChanged(dest string, body []byte) (bool, error) {
	remote := ParseList(body)
	if len(remote) == 0 {
		return false, fmt.Errorf("no subnet entries in remote body")
	}
	local, err := ParseFile(dest)
	if err == nil && CIDRSetsEqual(local, remote) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(dest, body, 0o644)
}

// RefreshFileIfChanged downloads url and updates dest when entries differ from the local file.
func RefreshFileIfChanged(url, dest string, client *http.Client) (bool, error) {
	if client == nil {
		client = netfetch.HTTPClient(netfetch.DefaultDNSAddr, "", 60*time.Second)
	}
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	changed, err := WriteListBodyIfChanged(dest, body)
	if err != nil {
		return false, fmt.Errorf("fetch %s: %w", url, err)
	}
	return changed, nil
}
