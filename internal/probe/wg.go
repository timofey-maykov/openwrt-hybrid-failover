package probe

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultWGHandshakeMaxAge is how old a WG/AWG handshake may be before the tunnel is unhealthy.
const DefaultWGHandshakeMaxAge = 3 * time.Minute

// WgHandshakeFresh reports whether iface is a healthy WireGuard/AmneziaWG tunnel.
// Non-WG interfaces return (true, "") so HTTP probes remain the health signal.
func WgHandshakeFresh(iface string, maxAge time.Duration) (bool, string) {
	st, wg := checkWGHandshake(iface, maxAge)
	if !wg {
		return true, ""
	}
	if st.fresh {
		return true, ""
	}
	return false, st.detail
}

func wgShow(iface string, args ...string) ([]byte, error) {
	for _, bin := range []string{"awg", "wg"} {
		cmdArgs := append([]string{"show", iface}, args...)
		out, err := exec.Command(bin, cmdArgs...).CombinedOutput()
		if err == nil {
			return out, nil
		}
	}
	return nil, fmt.Errorf("wg show %s: not available", iface)
}

func parseLatestHandshake(output []byte) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ts, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
		if err != nil || ts <= 0 {
			continue
		}
		t := time.Unix(ts, 0)
		if !found || t.After(latest) {
			latest = t
			found = true
		}
	}
	return latest, found
}

func formatDurationShort(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	if h < 48 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dd", h/24)
}
