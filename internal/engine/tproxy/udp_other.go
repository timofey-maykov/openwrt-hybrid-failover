//go:build !linux

package tproxy

import (
	"context"
	"net"
)

func listenUDPTransparent(port int) (*net.UDPConn, error) {
	return nil, net.ErrClosed
}

func (s *Server) serveUDP(ctx context.Context, conn *net.UDPConn) {}
