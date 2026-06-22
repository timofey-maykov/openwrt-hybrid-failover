// Package memlimit caps Go heap growth on small OpenWrt routers.
package memlimit

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

const defaultBytes = 64 << 20

func init() {
	limit := defaultBytes
	if raw := strings.TrimSpace(os.Getenv("GOMEMLIMIT")); raw != "" {
		if n, ok := parseLimit(raw); ok && n > 0 {
			limit = n
		}
	}
	debug.SetMemoryLimit(int64(limit))

	if strings.TrimSpace(os.Getenv("GOMAXPROCS")) == "" {
		runtime.GOMAXPROCS(1)
	}
}

func parseLimit(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	mult := 1
	switch {
	case strings.HasSuffix(raw, "GiB"):
		mult = 1 << 30
		raw = strings.TrimSuffix(raw, "GiB")
	case strings.HasSuffix(raw, "MiB"):
		mult = 1 << 20
		raw = strings.TrimSuffix(raw, "MiB")
	case strings.HasSuffix(raw, "KiB"):
		mult = 1 << 10
		raw = strings.TrimSuffix(raw, "KiB")
	case strings.HasSuffix(raw, "G"):
		mult = 1 << 30
		raw = strings.TrimSuffix(raw, "G")
	case strings.HasSuffix(raw, "M"):
		mult = 1 << 20
		raw = strings.TrimSuffix(raw, "M")
	case strings.HasSuffix(raw, "K"):
		mult = 1 << 10
		raw = strings.TrimSuffix(raw, "K")
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n * mult, true
}
