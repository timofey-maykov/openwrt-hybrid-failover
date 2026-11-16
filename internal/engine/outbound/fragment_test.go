package outbound

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

type captureFragmentConn struct {
	net.Conn
	writes [][]byte
}

func (c *captureFragmentConn) Write(p []byte) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), p...))
	return len(p), nil
}

func TestFragmentPreservesHelloAndFollowingRecords(t *testing.T) {
	opt, err := parseFinalMask(`{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"3","delay":"0"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	raw := &captureFragmentConn{}
	c := wrapFragment(raw, opt)
	hello := []byte{1, 0, 0, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	input := append([]byte{22, 3, 1, 0, byte(len(hello))}, hello...)
	tail := []byte{23, 3, 3, 0, 1, 99}
	input = append(input, tail...)
	if n, err := c.Write(input); err != nil || n != len(input) {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var got []byte
	for _, record := range raw.writes[:len(raw.writes)-1] {
		if len(record) < 5 || record[0] != 22 || int(binary.BigEndian.Uint16(record[3:5])) != len(record)-5 {
			t.Fatal("invalid TLS record")
		}
		got = append(got, record[5:]...)
	}
	if !bytes.Equal(got, hello) || !bytes.Equal(raw.writes[len(raw.writes)-1], tail) {
		t.Fatal("fragmentation changed the handshake or trailing data")
	}
	_, _ = c.Write(tail)
	if !bytes.Equal(raw.writes[len(raw.writes)-1], tail) {
		t.Fatal("application data was fragmented")
	}
}

func TestFragmentDelayCancelledByClose(t *testing.T) {
	opt, _ := parseFinalMask(`{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"3","interval":"1000"}}]}`)
	a, b := net.Pipe()
	defer b.Close()
	c := wrapFragment(a, opt)
	done := make(chan error, 1)
	go func() { _, err := c.Write([]byte{22, 3, 1, 0, 7, 1, 0, 0, 3, 1, 2, 3}); done <- err }()
	first := make([]byte, 8)
	if _, err := io.ReadFull(b, first); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled write succeeded")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fragment delay ignored close")
	}
}

func TestUnsupportedFinalMasksAreNotIgnored(t *testing.T) {
	for _, text := range []string{``, `null`, `{}`, `{"tcp":[{"type":"noise"}]}`, `{"tcp":[{"type":"fragment","settings":{"packets":"1-3","length":"10"}}]}`, `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"0"}}]}`, `{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"10-20","interval":"1001"}}]}`, helloFragment + `{}`} {
		if _, err := parseFinalMask(text); err == nil {
			t.Fatalf("unsupported mask accepted: %s", text)
		}
	}
}
