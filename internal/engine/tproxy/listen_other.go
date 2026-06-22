//go:build !linux

package tproxy

import "syscall"

func listenTransparent(network, address string, c syscall.RawConn) error {
	return nil
}
