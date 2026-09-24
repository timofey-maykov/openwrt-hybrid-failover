package dns

import (
	"net"
	"testing"
	"time"
)

// A socket that is still closing must not turn into a permanent failure: the
// watchdog restarts the engine, and Stop can return before the kernel has
// released the port.
func TestListenWithRetryWaitsForPortRelease(t *testing.T) {
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := probe.LocalAddr().String()

	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = probe.Close()
	}()

	start := time.Now()
	udpConn, tcpLn, err := listenWithRetry(addr)
	if err != nil {
		t.Fatalf("listenWithRetry: %v", err)
	}
	defer udpConn.Close()
	defer tcpLn.Close()

	if elapsed := time.Since(start); elapsed < 250*time.Millisecond {
		t.Errorf("returned after %v, expected it to wait for the port", elapsed)
	}
}

// A port somebody else keeps must still fail, just not instantly.
func TestListenWithRetryGivesUp(t *testing.T) {
	held, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold listen: %v", err)
	}
	defer held.Close()

	old := bindRetryForTest(200 * time.Millisecond)
	defer old()

	if _, _, err := listenWithRetry(held.LocalAddr().String()); err == nil {
		t.Fatal("expected an error while the address is held")
	}
}
