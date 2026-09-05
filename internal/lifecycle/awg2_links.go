package lifecycle

import (
	"fmt"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type awg2Link struct {
	peerSection string
	raw         string
	updateUCI   bool
}

type awg2PeerGroup struct {
	primary   awg2Link
	params    amnezia.AWG2Params
	endpoints []string
}

// SetupAWG2FromUCI scans routing sections for awg2:// (and vpn://→awg2) links and brings up interfaces.
// Multiple awg2 links with the same peer public key share one interface; alternate Host:Port
// values are kept for endpoint rotation when the current IP path dies.
// The second return value is true when at least one interface was created or reconfigured.
func SetupAWG2FromUCI(pkg *uci.Package) (bool, error) {
	if pkg == nil {
		return false, nil
	}
	var changed bool
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		groups, err := awg2PeerGroupsForSection(name, sec)
		if err != nil {
			return changed, err
		}
		for _, g := range groups {
			ifname := amnezia.AWG2InterfaceName(g.primary.peerSection)
			writeAWG2Endpoints(ifname, g.endpoints)
			_, synced, err := setupAWG2Interface(g.primary.peerSection, g.primary.raw, g.primary.updateUCI)
			if err != nil {
				return changed, fmt.Errorf("section %q peer %q: %w", name, g.primary.peerSection, err)
			}
			changed = changed || synced
		}
	}
	return changed, nil
}

func awg2PeerGroupsForSection(section string, sec *uci.Section) ([]awg2PeerGroup, error) {
	items := awg2LinksForSection(section, sec)
	byKey := make(map[string]*awg2PeerGroup)
	var order []string
	for _, item := range items {
		link, err := decodeProxyLink(item.raw)
		if err != nil {
			return nil, fmt.Errorf("section %q peer %q: %w", section, item.peerSection, err)
		}
		if !strings.HasPrefix(link, "awg2://") {
			continue
		}
		params, err := amnezia.ParseAWG2URI(link)
		if err != nil {
			return nil, fmt.Errorf("section %q peer %q: %w", section, item.peerSection, err)
		}
		ep := fmt.Sprintf("%s:%s", params.Host, params.Port)
		g := byKey[params.PublicKey]
		if g == nil {
			g = &awg2PeerGroup{
				primary:   awg2Link{peerSection: item.peerSection, raw: link, updateUCI: item.updateUCI},
				params:    params,
				endpoints: []string{ep},
			}
			byKey[params.PublicKey] = g
			order = append(order, params.PublicKey)
			continue
		}
		if !endpointInList(g.endpoints, ep) {
			g.endpoints = append(g.endpoints, ep)
		}
	}
	out := make([]awg2PeerGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out, nil
}

func endpointInList(list []string, ep string) bool {
	for _, v := range list {
		if v == ep {
			return true
		}
	}
	return false
}

func awg2LinksForSection(section string, sec *uci.Section) []awg2Link {
	var links []awg2Link
	if ps := strings.TrimSpace(sec.Get("proxy_string", "")); ps != "" {
		links = append(links, awg2Link{peerSection: section, raw: ps, updateUCI: true})
	}
	for i, link := range sec.GetList("failover_proxy_links") {
		links = append(links, awg2Link{
			peerSection: fmtPeerSection(section, i+1),
			raw:         link,
			updateUCI:   false,
		})
	}
	for i, link := range sec.GetList("urltest_proxy_links") {
		links = append(links, awg2Link{
			peerSection: fmtPeerSection(section, i+1),
			raw:         link,
			updateUCI:   false,
		})
	}
	return links
}

func fmtPeerSection(section string, index int) string {
	return fmt.Sprintf("%s-%d", section, index)
}

func decodeProxyLink(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "vpn://") {
		return amnezia.DecodeVPNURI(raw)
	}
	return raw, nil
}

func sectionProxyLinks(sec *uci.Section) []string {
	var links []string
	links = append(links, sec.GetList("failover_proxy_links")...)
	links = append(links, sec.GetList("urltest_proxy_links")...)
	if ps := sec.Get("proxy_string", ""); ps != "" {
		links = append(links, ps)
	}
	return links
}
