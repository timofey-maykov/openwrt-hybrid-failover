package plan

import (
	"os/exec"
	"strings"
)

// defaultWANInterface returns the IPv4 default route device (e.g. pppoe-wan).
func defaultWANInterface() string {
	out, err := exec.Command("ip", "-4", "route", "get", "1.0.0.1").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func proxyNeedsWANBind(kind OutboundKind) bool {
	switch kind {
	case OutboundVLESS, OutboundTrojan, OutboundHysteria2, OutboundShadowsocks, OutboundSocks:
		return true
	default:
		return false
	}
}
