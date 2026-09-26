//go:build linux

package outbound

import (
	"net"
	"syscall"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var bindErr error
		if err := c.Control(func(fd uintptr) {
			bindErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		}); err != nil {
			return err
		}
		return bindErr
	}
}

// engineSocketControl marks the socket with singbox.EngineSocketMark (see
// there for why) and, if iface is set, binds it to that interface.
func engineSocketControl(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var opErr error
		if err := c.Control(func(fd uintptr) {
			// SO_MARK needs CAP_NET_ADMIN. The engine runs as root on the
			// router; unprivileged runs (tests, CI) just go without the mark.
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, singbox.EngineSocketMark); err != nil && err != syscall.EPERM {
				opErr = err
				return
			}
			if iface != "" {
				opErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
			}
		}); err != nil {
			return err
		}
		return opErr
	}
}

func setReuseAddr(c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
}

func dialTCPBind(ctx net.Conn, err error) (net.Conn, error) {
	return ctx, err
}
