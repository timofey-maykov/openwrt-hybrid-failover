package subnets

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
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

// EnsureFile downloads url into dest when dest is missing, empty, or outdated vs remote.
func EnsureFile(url, dest string) error {
	changed, err := RefreshFileIfChanged(url, dest, nil)
	if err != nil {
		if cidrs, err2 := ParseFile(dest); err2 == nil && len(cidrs) > 0 {
			return nil
		}
		return err
	}
	if changed {
		return nil
	}
	if cidrs, err := ParseFile(dest); err == nil && len(cidrs) > 0 {
		return nil
	}
	return fmt.Errorf("fetch %s: no local subnet entries", url)
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
