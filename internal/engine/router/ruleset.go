package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
)

type fakeIPStore struct {
	mu       sync.Mutex
	network  *net.IPNet
	next     net.IP
	mapByIP  map[string]string
	mapByDom map[string]string
}

func newFakeIPStore(cidr string) *fakeIPStore {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		_, n, _ = net.ParseCIDR("198.18.0.0/15")
	}
	ip := n.IP.To4()
	next := make(net.IP, len(ip))
	copy(next, ip)
	next[2] = 1
	return &fakeIPStore{
		network:  n,
		next:     next,
		mapByIP:  make(map[string]string),
		mapByDom: make(map[string]string),
	}
}

func (f *fakeIPStore) Allocate(domain string) (string, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if ip, ok := f.mapByDom[domain]; ok {
		return ip, true
	}
	for {
		ip := make(net.IP, len(f.next))
		copy(ip, f.next)
		if !f.network.Contains(ip) {
			return "", false
		}
		f.incr()
		ips := ip.String()
		if _, used := f.mapByIP[ips]; used {
			continue
		}
		f.mapByIP[ips] = domain
		f.mapByDom[domain] = ips
		return ips, true
	}
}

func (f *fakeIPStore) LookupDomain(ip string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.mapByIP[ip]
	return d, ok
}

func (f *fakeIPStore) incr() {
	for i := len(f.next) - 1; i >= 0; i-- {
		f.next[i]++
		if f.next[i] != 0 {
			break
		}
	}
}

func (r *Router) loadRuleSet(rs plan.RuleSet) error {
	if r.domainSetsMap == nil {
		r.domainSetsMap = make(map[string][]string)
	}
	if r.subnetSets == nil {
		r.subnetSets = make(map[string][]*net.IPNet)
	}
	switch rs.Kind {
	case "domains":
		domains, err := loadDomains(rs)
		if err != nil {
			return err
		}
		r.domainSetsMap[rs.Tag] = domains
	case "subnets":
		nets, err := parseSubnetList(rs.Subnets)
		if err != nil {
			return err
		}
		r.subnetSets[rs.Tag] = nets
	}
	return nil
}

func loadDomains(rs plan.RuleSet) ([]string, error) {
	if len(rs.Domains) > 0 {
		return rs.Domains, nil
	}
	if rs.Path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(rs.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}
	var doc struct {
		Rules []struct {
			Domain       []string `json:"domain"`
			DomainSuffix []string `json:"domain_suffix"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil
	}
	var out []string
	for _, rule := range doc.Rules {
		out = append(out, rule.Domain...)
		out = append(out, rule.DomainSuffix...)
	}
	return out, nil
}

func parseSubnetList(cidrs []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, c := range subnets.NormalizeCIDRS(cidrs) {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			return nil, fmt.Errorf("parse cidr %q: %w", c, err)
		}
		out = append(out, n)
	}
	return out, nil
}
