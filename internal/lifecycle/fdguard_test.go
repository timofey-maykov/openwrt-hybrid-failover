package lifecycle

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestIsFDExhausted(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"wrapped sentinel", fmt.Errorf("nft restore: %w", syscall.EMFILE), true},
		{"system-wide sentinel", fmt.Errorf("listen: %w", syscall.ENFILE), true},
		{"path error", &os.PathError{Op: "open", Path: "/dev/null", Err: syscall.EMFILE}, true},
		{"flattened message", fmt.Errorf("ubus dump: open /dev/null: too many open files"), true},
		{"unrelated", fmt.Errorf("connection refused"), false},
	}
	for _, c := range cases {
		if got := isFDExhausted(c.err); got != c.want {
			t.Errorf("%s: isFDExhausted = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFDUsage(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("no /proc/self/fd on this host; fdUsage only reports on the Linux target")
	}
	open, limit := fdUsage()
	if open <= 0 {
		t.Errorf("open = %d, want > 0", open)
	}
	if limit == 0 {
		t.Errorf("limit = 0, want the soft RLIMIT_NOFILE")
	}
	if uint64(open) > limit {
		t.Errorf("open = %d exceeds limit = %d", open, limit)
	}
}
