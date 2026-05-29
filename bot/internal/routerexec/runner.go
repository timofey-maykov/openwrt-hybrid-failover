package routerexec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	timeout time.Duration
}

func New(timeout time.Duration) Runner {
	return Runner{timeout: timeout}
}

const coreBinary = "/usr/sbin/hybrid-failover"

func (r Runner) Run(ctx context.Context, name string, args ...string) (string, error) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r Runner) RunCoreRPC(ctx context.Context, method string) (string, error) {
	return r.Run(ctx, coreBinary, "rpc", method)
}
