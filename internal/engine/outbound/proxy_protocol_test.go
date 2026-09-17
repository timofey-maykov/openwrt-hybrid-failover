package outbound

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/sing-shadowsocks/shadowaead"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestSOCKS5TCPHandshakeAndData(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	serverErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer c.Close()
		br := bufio.NewReader(c)
		greeting := make([]byte, 3)
		if _, err = io.ReadFull(br, greeting); err != nil {
			serverErr <- err
			return
		}
		if string(greeting) != string([]byte{5, 1, 0}) {
			serverErr <- &protocolTestError{"unexpected greeting"}
			return
		}
		if _, err = c.Write([]byte{5, 0}); err != nil {
			serverErr <- err
			return
		}
		header := make([]byte, 4)
		if _, err = io.ReadFull(br, header); err != nil {
			serverErr <- err
			return
		}
		if header[0] != 5 || header[1] != 1 || header[3] != 3 {
			serverErr <- &protocolTestError{"unexpected connect request"}
			return
		}
		nameLen, err := br.ReadByte()
		if err != nil {
			serverErr <- err
			return
		}
		target := make([]byte, int(nameLen)+2)
		if _, err = io.ReadFull(br, target); err != nil {
			serverErr <- err
			return
		}
		if string(target[:nameLen]) != "target.example" || int(target[nameLen])<<8|int(target[nameLen+1]) != 8080 {
			serverErr <- &protocolTestError{"destination was not preserved"}
			return
		}
		if _, err = c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 1}); err != nil {
			serverErr <- err
			return
		}
		payload := make([]byte, 4)
		if _, err = io.ReadFull(br, payload); err == nil {
			_, err = c.Write(payload)
		}
		serverErr <- err
	}()

	h, err := newProxyHandler(plan.OutboundPlan{Tag: "socks", Kind: plan.OutboundSocks, ProxyURI: "socks5://" + ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, err := h.DialTCP(ctx, "tcp", "target.example:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	if _, err = io.ReadFull(c, reply); err != nil || string(reply) != "ping" {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
}

type protocolTestError struct{ message string }

func (e *protocolTestError) Error() string { return e.message }

type echoShadowHandler struct {
	t           *testing.T
	destination chan M.Socksaddr
}

func (h *echoShadowHandler) NewConnection(_ context.Context, conn net.Conn, metadata M.Metadata) error {
	h.destination <- metadata.Destination
	_, err := io.Copy(conn, conn)
	return err
}

func (h *echoShadowHandler) NewPacketConnection(context.Context, N.PacketConn, M.Metadata) error {
	return nil
}

func (h *echoShadowHandler) NewError(context.Context, error) {}

func TestShadowsocksTCPEncryptionAndDestination(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	handler := &echoShadowHandler{t: t, destination: make(chan M.Socksaddr, 1)}
	service, err := shadowaead.NewService("aes-128-gcm", nil, "secret", 30, handler)
	if err != nil {
		t.Fatal(err)
	}
	serverErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			err = service.NewConnection(context.Background(), c, M.Metadata{})
		}
		serverErr <- err
	}()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	h, err := newProxyHandler(plan.OutboundPlan{Tag: "ss", Kind: plan.OutboundShadowsocks, ProxyURI: "ss://aes-128-gcm:secret@127.0.0.1:" + port})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, err := h.DialTCP(ctx, "tcp", "shadow.example:443")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 5)
	if _, err = io.ReadFull(c, reply); err != nil || string(reply) != "hello" {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
	if got := <-handler.destination; got.String() != "shadow.example:443" {
		t.Fatalf("destination=%s", got)
	}
	_ = c.Close()
	select {
	case <-serverErr:
	case <-time.After(time.Second):
		t.Fatal("server did not finish")
	}
}

func TestTrojanRequestAndUDPFraming(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- writeTrojanRequest(client, "secret", 1, parseDestAddr("trojan.example:443"))
	}()
	hash := sha256.Sum224([]byte("secret"))
	wantPrefix := hex.EncodeToString(hash[:]) + "\r\n"
	prefix := make([]byte, len(wantPrefix))
	if _, err := io.ReadFull(server, prefix); err != nil || string(prefix) != wantPrefix {
		t.Fatalf("prefix=%q err=%v", prefix, err)
	}
	command := make([]byte, 1)
	if _, err := io.ReadFull(server, command); err != nil || command[0] != 1 {
		t.Fatalf("command=%v err=%v", command, err)
	}
	dest, err := M.SocksaddrSerializer.ReadAddrPort(server)
	if err != nil || dest.String() != "trojan.example:443" {
		t.Fatalf("destination=%s err=%v", dest, err)
	}
	ending := make([]byte, 2)
	if _, err = io.ReadFull(server, ending); err != nil || string(ending) != "\r\n" {
		t.Fatalf("ending=%q err=%v", ending, err)
	}
	if err = <-errCh; err != nil {
		t.Fatal(err)
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	writer := &trojanPacketConn{Conn: left}
	reader := &trojanPacketConn{Conn: right}
	writeErr := make(chan error, 1)
	go func() {
		_, err := writer.WriteTo([]byte("datagram"), parseDestAddr("udp.example:53"))
		writeErr <- err
	}()
	buf := make([]byte, 32)
	n, addr, err := reader.ReadFrom(buf)
	if err != nil || string(buf[:n]) != "datagram" || addr.String() != "udp.example:53" {
		t.Fatalf("payload=%q addr=%v err=%v", buf[:n], addr, err)
	}
	if err = <-writeErr; err != nil {
		t.Fatal(err)
	}
}
