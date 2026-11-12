package subscription

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
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

	if _, err := uci.Exec("delete", uciKey); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
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
	// Names are the channel_names entries (id=Name) the refresh wrote, so the
	// next one replaces them and leaves names given by hand alone.
	Names []string `json:"names,omitempty"`
}

func loadOwned(section string) []string {
	return loadOwnedState(section).Links
}

func loadOwnedState(section string) ownedLinks {
	raw, err := os.ReadFile(paths.SubscriptionState)
	if err != nil {
		return ownedLinks{}
	}
	var st ownedLinks
	if json.Unmarshal(raw, &st) != nil || st.Section != section {
		return ownedLinks{}
	}
	return st
}

func saveOwned(section string, links []string) {
	saveOwnedState(ownedLinks{Section: section, Links: links, Names: loadOwnedState(section).Names})
}

func saveOwnedState(st ownedLinks) {
	raw, err := json.Marshal(st)
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

// ApplyLinks writes the links like ApplyToUCI and names their channels after
// the panel's server names (channel_names, id=Name). Names for subscription
// channels follow the panel; names of hand-added channels stay.
func ApplyLinks(uciPath, mainSection string, links []Link, merge bool) (listKey string, err error) {
	uris := make([]string, 0, len(links))
	for _, l := range links {
		uris = append(uris, l.URI)
	}
	listKey, err = ApplyToUCI(uciPath, mainSection, uris, merge)
	if err != nil {
		return "", err
	}
	if err := applyNames(uciPath, mainSection, links); err != nil {
		return listKey, err
	}
	return listKey, nil
}

func applyNames(uciPath, mainSection string, links []Link) error {
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return err
	}
	sec := pkg.Section(mainSection)
	if sec == nil {
		return fmt.Errorf("section not found")
	}
	st := loadOwnedState(mainSection)
	final, owned := planNames(sec.GetList("channel_names"), st.Names, links)
	key := paths.UCIPackage + "." + mainSection + ".channel_names"
	if _, err := uci.Exec("delete", key); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
		return fmt.Errorf("uci delete %s: %w", key, err)
	}
	for _, entry := range final {
		if _, err := uci.Exec("add_list", key+"="+entry); err != nil {
			return fmt.Errorf("uci add_list %s: %w", key, err)
		}
	}
	if _, err := uci.Exec("commit", paths.UCIPackage); err != nil {
		return fmt.Errorf("uci commit: %w", err)
	}
	st.Section, st.Names = mainSection, owned
	if st.Links == nil {
		st.Links = loadOwned(mainSection)
	}
	saveOwnedState(st)
	return nil
}

// planNames merges the panel's server names into channel_names.
//
// Channels present in the subscription take the panel name (same channel id
// as channels.ID). That replaces a stale label left from an earlier name of
// the same peer, and also names the refresh previously wrote.
// Names for other channel ids (hand-added links) stay. Names the previous
// refresh owned for peers that left the subscription are dropped.
func planNames(existing, ownedNames []string, links []Link) (final, owned []string) {
	subName := map[string]string{}
	var subOrder []string
	for _, l := range links {
		if l.Name == "" {
			continue
		}
		id := channels.ID(l.URI)
		if _, dup := subName[id]; dup {
			continue
		}
		subName[id] = l.Name
		subOrder = append(subOrder, id)
	}
	ownedIDs := map[string]struct{}{}
	for _, entry := range ownedNames {
		if id, _, ok := strings.Cut(entry, "="); ok {
			ownedIDs[strings.TrimSpace(id)] = struct{}{}
		}
	}
	for _, entry := range existing {
		id, _, ok := strings.Cut(entry, "=")
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			continue
		}
		if _, fromSub := subName[id]; fromSub {
			continue
		}
		if _, wasOwned := ownedIDs[id]; wasOwned {
			continue
		}
		final = append(final, entry)
	}
	for _, id := range subOrder {
		entry := id + "=" + subName[id]
		final = append(final, entry)
		owned = append(owned, entry)
	}
	return final, owned
}
