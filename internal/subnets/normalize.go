package subnets

import (
	"net"
	"sort"
	"strings"
)

// NormalizeForNFT drops invalid, duplicate, and subset CIDRs for nft interval sets.
func NormalizeForNFT(cidrs []string) []string {
	type entry struct {
		raw   string
		ipnet *net.IPNet
	}
	var nets []entry
	seen := make(map[string]struct{})
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, ok := seen[cidr]; ok {
			continue
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if overlapsFakeIP(ipnet) {
			continue
		}
		seen[cidr] = struct{}{}
		nets = append(nets, entry{raw: cidr, ipnet: ipnet})
	}
	sort.Slice(nets, func(i, j int) bool {
		li, _ := nets[i].ipnet.Mask.Size()
		lj, _ := nets[j].ipnet.Mask.Size()
		if li != lj {
			return li < lj
		}
		return nets[i].raw < nets[j].raw
	})
	var out []string
	for _, n := range nets {
		covered := false
		for _, kept := range out {
			_, keptNet, _ := net.ParseCIDR(kept)
			if keptNet != nil && keptNet.Contains(n.ipnet.IP) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, n.raw)
		}
	}
	return out
}

func overlapsFakeIP(n *net.IPNet) bool {
	_, fake, _ := net.ParseCIDR("198.18.0.0/15")
	return fake.Contains(n.IP) || n.Contains(fake.IP)
}
