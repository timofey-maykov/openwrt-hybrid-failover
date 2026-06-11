package routerexec

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSH struct {
	timeout      time.Duration
	host         string
	port         int
	user         string
	identityFile string
}

func NewSSH(timeout time.Duration, host string, port int, user, identityFile string) SSH {
	if port <= 0 {
		port = 22
	}
	if user == "" {
		user = "root"
	}
	return SSH{
		timeout:      timeout,
		host:         host,
		port:         port,
		user:         user,
		identityFile: identityFile,
	}
}

func (r SSH) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := shellJoin(name, args...)
	return r.runShell(ctx, cmd)
}

func (r SSH) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	rpcArgs := append([]string{"rpc", method}, args...)
	return r.Run(ctx, coreBinary, rpcArgs...)
}

func (r SSH) runShell(ctx context.Context, cmd string) (string, error) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := r.dial(cctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer sess.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	if err := sess.Run(cmd); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("ssh %s: %s: %s", r.host, cmd, msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r SSH) dial(ctx context.Context) (*ssh.Client, error) {
	key, err := os.ReadFile(r.identityFile)
	if err != nil {
		return nil, fmt.Errorf("read identity_file %q: %w", r.identityFile, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse identity_file: %w", err)
	}

	cfg := &ssh.ClientConfig{
		User:            r.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // home routers, key in config path
		Timeout:         r.timeout,
	}
	addr := net.JoinHostPort(r.host, fmt.Sprintf("%d", r.port))

	type dialResult struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan dialResult, 1)
	go func() {
		c, err := ssh.Dial("tcp", addr, cfg)
		ch <- dialResult{c, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", addr, res.err)
		}
		return res.client, nil
	}
}

func shellJoin(name string, args ...string) string {
	parts := []string{shellQuote(name)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
