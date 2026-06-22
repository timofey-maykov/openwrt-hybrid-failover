//go:build !linux

package outbound

import (
	"syscall"
)

func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return nil
	}
}

func setReuseAddr(c syscall.RawConn) error {
	return nil
}
