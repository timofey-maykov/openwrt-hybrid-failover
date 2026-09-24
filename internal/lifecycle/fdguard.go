package lifecycle

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

// fdUsage reports how many descriptors the process currently holds and the
// soft RLIMIT_NOFILE it is allowed. A zero limit means the rlimit could not
// be read.
func fdUsage() (open int, limit uint64) {
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		// ReadDir itself holds one descriptor while it runs.
		open = len(entries) - 1
		if open < 0 {
			open = 0
		}
	}
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl); err == nil {
		limit = uint64(rl.Cur)
	}
	return open, limit
}

// isFDExhausted reports whether err was caused by this process running out of
// file descriptors.
//
// This matters because the engine restart the watchdog normally performs
// cannot clear it: the descriptor table belongs to the process, so a restarted
// engine fails to open its sockets just as the old one did and the watchdog
// spins until something else frees descriptors. Letting procd respawn us is
// the only bounded way back.
//
// Most call sites wrap with %v rather than %w, so the sentinel check alone is
// not enough and the message is matched as well.
func isFDExhausted(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
		return true
	}
	return strings.Contains(err.Error(), "too many open files")
}
