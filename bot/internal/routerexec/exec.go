package routerexec

import (
	"context"
	"strings"
	"time"
)

// Exec runs commands on a hybrid-failover host (local OpenWrt or remote via SSH).
type Exec interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
	RunCoreRPC(ctx context.Context, method string, args ...string) (string, error)
}

// DefaultCommandTimeout bounds a command when the caller's context has no deadline.
const DefaultCommandTimeout = 30 * time.Second

// withDeadline keeps a deadline the caller already set (long operations such as
// restart or list-update set their own) and otherwise applies fallback.
func withDeadline(ctx context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	if fallback <= 0 {
		fallback = DefaultCommandTimeout
	}
	return context.WithTimeout(ctx, fallback)
}

// failureDetail picks the most useful text for an error: core RPC methods
// report failures as JSON on stdout, shell tools write to stderr.
func failureDetail(stdout, stderr string) string {
	if s := strings.TrimSpace(stderr); s != "" {
		return s
	}
	return strings.TrimSpace(stdout)
}
