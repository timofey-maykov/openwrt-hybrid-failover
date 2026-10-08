package routerexec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Local struct {
	timeout time.Duration
}

func NewLocal(timeout time.Duration) Local {
	return Local{timeout: timeout}
}

const coreBinary = "/usr/sbin/hybrid-failover"

func (r Local) Run(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := withDeadline(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("timeout: %w", err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, failureDetail(stdout.String(), stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunBytes returns the command's stdout unchanged (binary safe).
func (r Local) RunBytes(ctx context.Context, name string, args ...string) ([]byte, error) {
	cctx, cancel := withDeadline(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("timeout: %w", err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, failureDetail(stdout.String(), stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (r Local) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	rpcArgs := append([]string{"rpc", method}, args...)
	return r.Run(ctx, coreBinary, rpcArgs...)
}
