package subscription

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func listKeyForSection(sec *uci.Section) (string, error) {
	if sec == nil {
		return "", fmt.Errorf("section not found")
	}
	switch sec.Get("connection_type", "") {
	case "vpn":
		return "failover_proxy_links", nil
	case "proxy":
		if sec.Get("proxy_config_type", "url") == "urltest" {
			return "urltest_proxy_links", nil
		}
		return "", fmt.Errorf("proxy section requires proxy_config_type=urltest for subscription links")
	default:
		return "", fmt.Errorf("connection_type %q does not support subscription links", sec.Get("connection_type", ""))
	}
}

// ApplyToUCI writes links to failover_proxy_links or urltest_proxy_links on mainSection.
// When merge is true, existing list entries are kept and new unique links are appended.
// Links that the previous refresh wrote are replaced, so servers dropped from the
// subscription disappear and the links added by hand stay.
func ApplyToUCI(uciPath, mainSection string, links []string, merge bool) (listKey string, err error) {
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return "", err
	}
	sec := pkg.Section(mainSection)
	listKey, err = listKeyForSection(sec)
	if err != nil {
		return "", err
	}

	uciKey := paths.UCIPackage + "." + mainSection + "." + listKey
	finalLinks := links
	if merge {
		kept := dropOwned(sec.GetList(listKey), loadOwned(mainSection))
		finalLinks = mergeLinks(kept, links)
	}

	if _, err := uci.Exec("delete", uciKey); err != nil && !strings.Contains(err.Error(), "Not found") {
		return "", fmt.Errorf("uci delete %s: %w", uciKey, err)
	}
	for _, link := range finalLinks {
		if _, err := uci.Exec("add_list", uciKey+"="+link); err != nil {
			return "", fmt.Errorf("uci add_list %s: %w", uciKey, err)
		}
	}
	if _, err := uci.Exec("commit", paths.UCIPackage); err != nil {
		return "", fmt.Errorf("uci commit: %w", err)
	}
	if merge {
		saveOwned(mainSection, links)
	}
	return listKey, nil
}

type ownedLinks struct {
	Section string   `json:"section"`
	Links   []string `json:"links"`
}

func loadOwned(section string) []string {
	raw, err := os.ReadFile(paths.SubscriptionState)
	if err != nil {
		return nil
	}
	var st ownedLinks
	if json.Unmarshal(raw, &st) != nil || st.Section != section {
		return nil
	}
	return st.Links
}

func saveOwned(section string, links []string) {
	raw, err := json.Marshal(ownedLinks{Section: section, Links: links})
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(paths.SubscriptionState), 0o755) != nil {
		return
	}
	_ = os.WriteFile(paths.SubscriptionState, raw, 0o600)
}

func dropOwned(existing, owned []string) []string {
	if len(owned) == 0 {
		return existing
	}
	drop := make(map[string]struct{}, len(owned))
	for _, l := range owned {
		drop[strings.TrimSpace(l)] = struct{}{}
	}
	out := make([]string, 0, len(existing))
	for _, l := range existing {
		if _, ok := drop[strings.TrimSpace(l)]; !ok {
			out = append(out, l)
		}
	}
	return out
}

func mergeLinks(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, link := range existing {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}
	for _, link := range incoming {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}
	return out
}
