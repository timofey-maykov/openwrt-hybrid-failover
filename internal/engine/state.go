package engine

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

const runningStatePath = "/var/run/hybrid-failover/engine.running"

func markRunningState(running bool) {
	if running {
		_ = os.MkdirAll("/var/run/hybrid-failover", 0o755)
		_ = os.WriteFile(runningStatePath, []byte("1\n"), 0o644)
		return
	}
	_ = os.Remove(runningStatePath)
}

// Alive reports whether the native engine serves tproxy and DNS on expected ports.
func Alive() bool {
	return probesHealthy()
}

func probesHealthy() bool {
	return probeEngineTPROXY() && DNSReady()
}

// DNSReady reports whether the native engine DNS listener accepts TCP on
// 127.0.0.42:53.
//
// Something answering there is not enough: dnsmasq with bind-dynamic can grab
// the address when it (re)starts while the engine is down, and then it answers
// too. Taking that for the engine kept the "stop dnsmasq" path from ever
// running, and the engine failed its bind every few seconds until someone
// restarted dnsmasq by hand. So when /proc tells who holds the listener, only
// the engine counts.
func DNSReady() bool {
	if !probeTCP(plan.DNSListenAddr, plan.DNSListenPort) {
		return false
	}
	if comm, ok := dnsListenerOwner(plan.DNSListenAddr, plan.DNSListenPort); ok {
		return isEngineComm(comm)
	}
	return true
}

// TPROXY listeners do not accept plain TCP connects on 127.0.0.1:1602.
func probeEngineTPROXY() bool {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: plan.TPROXYPort, IP: net.IPv6zero})
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

func probeTCP(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
