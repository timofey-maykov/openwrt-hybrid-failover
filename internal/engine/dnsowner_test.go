package engine

import (
	"os"
	"path/filepath"
	"testing"
)

const tcpHeader = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"

// fakeProc builds /proc/net/tcp with a listener on 127.0.0.42:53 (inode
// 4242) and one process per entry in owners, pid -> comm, where the first
// one holds that socket.
func fakeProc(t *testing.T, listen bool, holder string) {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "net"), 0o755))
	tcp := tcpHeader +
		"   0: 0100007F:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1111 1 0 100 0 0 10 0\n"
	if listen {
		tcp += "   1: 2A00007F:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 4242 1 0 100 0 0 10 0\n"
	}
	must(os.WriteFile(filepath.Join(root, "net", "tcp"), []byte(tcp), 0o644))

	mk := func(pid, comm string, inodes ...string) {
		fd := filepath.Join(root, pid, "fd")
		must(os.MkdirAll(fd, 0o755))
		must(os.WriteFile(filepath.Join(root, pid, "comm"), []byte(comm+"\n"), 0o644))
		for i, ino := range inodes {
			must(os.Symlink("socket:["+ino+"]", filepath.Join(fd, string(rune('3'+i)))))
		}
	}
	mk("1", "procd")
	switch holder {
	case "dnsmasq":
		mk("4830", "dnsmasq", "1111", "4242")
		mk("4300", "hybrid-failover", "9999")
	case "engine":
		mk("4830", "dnsmasq", "1111")
		mk("4300", "hybrid-failover", "9999", "4242")
	}
	old := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = old })
}

func TestDNSListenerOwnerDnsmasq(t *testing.T) {
	fakeProc(t, true, "dnsmasq")
	comm, ok := dnsListenerOwner("127.0.0.42", 53)
	if !ok || comm != "dnsmasq" || isEngineComm(comm) {
		t.Fatalf("got %q %v, want dnsmasq and not the engine", comm, ok)
	}
}

func TestDNSListenerOwnerEngine(t *testing.T) {
	fakeProc(t, true, "engine")
	comm, ok := dnsListenerOwner("127.0.0.42", 53)
	if !ok || !isEngineComm(comm) {
		t.Fatalf("got %q %v, want the engine", comm, ok)
	}
}

func TestDNSListenerOwnerUnknown(t *testing.T) {
	fakeProc(t, false, "")
	if comm, ok := dnsListenerOwner("127.0.0.42", 53); ok {
		t.Fatalf("got %q, want no owner when nothing listens", comm)
	}
}

func TestIsEngineComm(t *testing.T) {
	for comm, want := range map[string]bool{
		"hybrid-failover": true,
		"hybrid-failove":  true,
		"dnsmasq":         false,
		"hybrid":          false,
		"":                false,
	} {
		if got := isEngineComm(comm); got != want {
			t.Errorf("isEngineComm(%q) = %v, want %v", comm, got, want)
		}
	}
}
