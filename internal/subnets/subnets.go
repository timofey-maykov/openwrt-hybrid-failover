package subnets

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ParseList reads one CIDR per line from a .lst file body.
func ParseList(data []byte) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, _, err := net.ParseCIDR(line); err != nil {
			if ip := net.ParseIP(line); ip != nil && ip.To4() != nil {
				line += "/32"
			} else {
				continue
			}
		}
		out = append(out, line)
	}
	return out
}

// ParseFile reads CIDRs from a .lst file path.
func ParseFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseList(data), nil
}

// EnsureFile downloads url into dest when dest is missing or empty.
func EnsureFile(url, dest string) error {
	if cidrs, err := ParseFile(dest); err == nil && len(cidrs) > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(ParseList(body)) == 0 {
		return fmt.Errorf("fetch %s: no subnet entries", url)
	}
	return os.WriteFile(dest, body, 0o644)
}

// SplitItems splits comma/newline/space separated CIDRs.
func SplitItems(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == ' '
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
