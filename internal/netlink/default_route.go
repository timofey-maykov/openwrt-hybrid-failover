package netlink

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// HasIPv4DefaultRoute reports whether the main table has an IPv4 default route.
func HasIPv4DefaultRoute() bool {
	out, err := exec.Command("ip", "-4", "route", "show", "default").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

type ubusIfaceDump struct {
	Interface []ubusIface `json:"interface"`
}

type ubusIface struct {
	Interface string      `json:"interface"`
	Up        bool        `json:"up"`
	L3Device  string      `json:"l3_device"`
	Metric    int         `json:"metric"`
	Route     []ubusRoute `json:"route"`
}

type ubusRoute struct {
	Target  string `json:"target"`
	Mask    int    `json:"mask"`
	Nexthop string `json:"nexthop"`
}

// EnsureIPv4DefaultRoute reinstalls a missing IPv4 default from netifd interface state.
// Prefer PPPoE/L2TP-style WAN over static underlay gateways when both claim default.
// Returns true when a route was installed.
func EnsureIPv4DefaultRoute() (bool, error) {
	if HasIPv4DefaultRoute() {
		return false, nil
	}
	out, err := exec.Command("ubus", "call", "network.interface", "dump").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("ubus network.interface dump: %w: %s", err, strings.TrimSpace(string(out)))
	}
	cand, ok := pickDefaultRouteCandidate(out)
	if !ok {
		return false, fmt.Errorf("no IPv4 default candidate in netifd")
	}
	args := []string{"route", "replace", "default"}
	if cand.nexthop != "" && cand.nexthop != "0.0.0.0" {
		args = append(args, "via", cand.nexthop)
	}
	args = append(args, "dev", cand.dev)
	if cand.metric > 0 {
		args = append(args, "metric", fmt.Sprintf("%d", cand.metric))
	}
	if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		return false, fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

type defaultRouteCandidate struct {
	dev     string
	nexthop string
	metric  int
	score   int
}

func pickDefaultRouteCandidate(ubusJSON []byte) (defaultRouteCandidate, bool) {
	var dump ubusIfaceDump
	if err := json.Unmarshal(ubusJSON, &dump); err != nil {
		return defaultRouteCandidate{}, false
	}
	best := defaultRouteCandidate{score: -1}
	found := false
	for _, iface := range dump.Interface {
		if !iface.Up || iface.L3Device == "" {
			continue
		}
		for _, r := range iface.Route {
			if r.Target != "0.0.0.0" || r.Mask != 0 {
				continue
			}
			score := scoreDefaultCandidate(iface.L3Device, iface.Interface)
			// Lower metric wins when score ties.
			if !found || score > best.score || (score == best.score && iface.Metric < best.metric) {
				best = defaultRouteCandidate{
					dev:     iface.L3Device,
					nexthop: r.Nexthop,
					metric:  iface.Metric,
					score:   score,
				}
				found = true
			}
		}
	}
	return best, found
}

func scoreDefaultCandidate(l3, name string) int {
	l3 = strings.ToLower(l3)
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(l3, "pppoe-"), strings.HasPrefix(l3, "l2tp-"), strings.HasPrefix(l3, "pptp-"):
		return 100
	case strings.Contains(name, "vpn"), strings.Contains(name, "ppp"):
		return 80
	case strings.HasPrefix(l3, "wan"), name == "wan":
		return 20
	default:
		return 50
	}
}
