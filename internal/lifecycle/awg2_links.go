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

// SetupAWG2FromUCI scans routing sections for awg2:// (and vpn://→awg2) links and brings up interfaces.
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
		for _, item := range awg2LinksForSection(name, sec) {
			link, err := decodeProxyLink(item.raw)
			if err != nil {
				return changed, fmt.Errorf("section %q peer %q: %w", name, item.peerSection, err)
			}
			if !strings.HasPrefix(link, "awg2://") {
				continue
			}
			_, synced, err := setupAWG2Interface(item.peerSection, link, item.updateUCI)
			if err != nil {
				return changed, fmt.Errorf("section %q peer %q: %w", name, item.peerSection, err)
			}
			changed = changed || synced
		}
	}
	return changed, nil
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
