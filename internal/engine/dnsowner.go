package engine

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// procRoot is /proc; tests point it at a fake tree.
var procRoot = "/proc"

// engineComm is the process name of the engine (comm is cut at 15 bytes).
const engineComm = "hybrid-failover"

// dnsListenerOwner returns the command name of the process that holds the TCP
// listener on host:port, found through /proc/net/tcp and the socket inodes in
// /proc/<pid>/fd. ok is false when that cannot be determined (no /proc, no
// such listener, or no process found for it).
func dnsListenerOwner(host string, port int) (comm string, ok bool) {
	inode, found := tcpListenInode(host, port)
	if !found {
		return "", false
	}
	return socketOwner(inode)
}

func tcpListenInode(host string, port int) (string, bool) {
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return "", false
	}
	want := fmt.Sprintf("%02X%02X%02X%02X:%04X", ip[3], ip[2], ip[1], ip[0], port)
	f, err := os.Open(filepath.Join(procRoot, "net", "tcp"))
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// sl local_address rem_address st tx_queue:rx_queue tr:tm->when retrnsmt uid timeout inode
		if len(fields) < 10 || fields[1] != want || fields[3] != "0A" {
			continue
		}
		if fields[9] == "0" {
			continue
		}
		return fields[9], true
	}
	return "", false
}

func socketOwner(inode string) (string, bool) {
	target := "socket:[" + inode + "]"
	pids, err := os.ReadDir(procRoot)
	if err != nil {
		return "", false
	}
	for _, p := range pids {
		if _, err := strconv.Atoi(p.Name()); err != nil {
			continue
		}
		fdDir := filepath.Join(procRoot, p.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || link != target {
				continue
			}
			comm, err := os.ReadFile(filepath.Join(procRoot, p.Name(), "comm"))
			if err != nil {
				return "", false
			}
			return strings.TrimSpace(string(comm)), true
		}
	}
	return "", false
}

// isEngineComm matches the engine's comm, which the kernel cuts at 15 bytes.
func isEngineComm(comm string) bool {
	return strings.HasPrefix(engineComm, comm) && len(comm) >= len("hybrid-failove")
}
