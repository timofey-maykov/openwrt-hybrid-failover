package probe

import (
	"fmt"
	"time"
)

// AWGHandshake is WireGuard/AmneziaWG liveness for dashboard overlay.
type AWGHandshake struct {
	Present bool
	Fresh   bool
	Age     time.Duration
}

// OverlayAWG2Health explains AWG2 card status. HTTP delay is urltest ranking;
// handshake is the tunnel liveness users actually care about.
func OverlayAWG2Health(httpDelay int, hs AWGHandshake) (ok bool, delay int, detail string) {
	if httpDelay > 0 {
		return true, httpDelay, ""
	}
	if !hs.Present {
		return false, 0, "нет интерфейса AWG"
	}
	if hs.Fresh {
		return true, 0, "handshake есть, HTTP urltest не проходит"
	}
	if hs.Age > 0 {
		return false, 0, fmt.Sprintf("handshake устарел (%s)", formatDurationShort(hs.Age))
	}
	return false, 0, "нет handshake"
}

// ReadAWGHandshake reads latest handshake for iface. Missing iface is Present=false.
func ReadAWGHandshake(iface string) AWGHandshake {
	if iface == "" {
		return AWGHandshake{}
	}
	st, isWG := checkWGHandshake(iface, DefaultWGHandshakeMaxAge)
	if !isWG {
		return AWGHandshake{}
	}
	return AWGHandshake{Present: true, Fresh: st.fresh, Age: st.age}
}
