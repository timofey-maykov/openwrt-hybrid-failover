//go:build linux

package outbound

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestMissingVPNInterfaceFailsClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, err := outboundDialer("hf-missing0").DialContext(ctx, "tcp", ln.Addr().String())
	if err == nil {
		c.Close()
		t.Fatal("dial succeeded despite nonexistent VPN interface: SO_BINDTODEVICE failure was ignored")
	}
}
