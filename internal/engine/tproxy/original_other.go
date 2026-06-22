//go:build !linux

package tproxy

import (
	"fmt"
	"net"
)

func originalDstPlatform(conn *net.TCPConn) (originalDest, error) {
	return originalDest{}, fmt.Errorf("original dst unsupported")
}
