package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

const fakeipLookupTimeout = 2 * time.Second
const fakeipEngineWait = 8 * time.Second
const fakeipRetryAttempts = 6
const fakeipRetryDelay = 350 * time.Millisecond

func validateFakeIPResult(ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return fmt.Errorf("no fakeip response for %s", singbox.FAKEIPTestDomain)
	}
	if !strings.HasPrefix(ip, "198.18.") {
		return fmt.Errorf("unexpected fakeip address %q (want 198.18.x.x)", ip)
	}
	return nil
}

func isDNSTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "no such host") ||
		errors.Is(err, context.DeadlineExceeded)
}

func waitForEngineDNS(timeout time.Duration) error {
	if os.Getenv("HF_SKIP_ENGINE_WAIT") == "1" {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if engine.Alive() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("native engine DNS not ready")
}

// lookupFakeIP resolves singbox.FAKEIPTestDomain via resolverHost (e.g. 127.0.0.42).
func lookupFakeIP(ctx context.Context, resolverHost, qname string) (string, error) {
	resolverHost = strings.TrimSpace(resolverHost)
	if resolverHost == "" {
		return "", fmt.Errorf("empty resolver address")
	}
	addr := resolverHost
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "53")
	}

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = fakeipLookupTimeout
			return d.DialContext(ctx, "udp", addr)
		},
	}

	ips, err := r.LookupHost(ctx, qname)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if strings.HasPrefix(ip, "198.18.") {
			return ip, nil
		}
	}
	if len(ips) > 0 {
		return ips[0], nil
	}
	return "", nil
}

func lookupFakeIPWithRetry(ctx context.Context, resolverHost, qname string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < fakeipRetryAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		result, err := lookupFakeIP(ctx, resolverHost, qname)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isDNSTransient(err) || attempt+1 >= fakeipRetryAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(fakeipRetryDelay):
		}
	}
	return "", lastErr
}
