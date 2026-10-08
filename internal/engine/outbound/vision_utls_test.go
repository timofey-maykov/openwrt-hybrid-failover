//go:build with_utls

package outbound

import (
	"net"
	"testing"

	utls "github.com/metacubex/utls"
	"github.com/sagernet/sing-vmess/vless"
)

// VLESS with flow xtls-rprx-vision reaches into the TLS connection by type.
// sing-vmess knows uTLS connections only when it is built with the with_utls
// tag; without it the REALITY connection is rejected with "vision: not a valid
// supported TLS connection: *outbound.realityConnWrapper".
func TestVisionAcceptsRealityConnection(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "example.com"}, utls.HelloChrome_Auto)
	w := &realityConnWrapper{UConn: uc}
	var id [16]byte
	if _, err := vless.NewVisionConn(w, w, id, newNopLogger()); err != nil {
		t.Fatalf("vision refuses the REALITY connection: %v", err)
	}
}
