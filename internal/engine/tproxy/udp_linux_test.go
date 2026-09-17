//go:build linux

package tproxy

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

func TestDialUDPReplyUsesOriginalSource(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("IP_TRANSPARENT requires root or CAP_NET_ADMIN")
	}
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reserved, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.2")})
	if err != nil {
		t.Fatal(err)
	}
	original := reserved.LocalAddr().(*net.UDPAddr)
	_ = reserved.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply, err := dialUDPReply(ctx, original, client.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer reply.Close()
	if _, err = reply.Write([]byte("reply")); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 16)
	n, source, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "reply" || !source.IP.Equal(original.IP) || source.Port != original.Port {
		t.Fatalf("payload=%q source=%s want=%s", buf[:n], source, original)
	}
}
