//go:build !linux

package outbound

import (
	"fmt"
	"syscall"
)

func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return fmt.Errorf("binding to interface %q is only supported on Linux", iface)
	}
}

func setReuseAddr(c syscall.RawConn) error {
	return nil
}
