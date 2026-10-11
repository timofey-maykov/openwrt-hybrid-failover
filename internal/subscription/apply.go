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
// the panel's server names (channel_names, id=Name). Names the user set by
// hand stay; names from the previous refresh are replaced.
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

// planNames merges the panel's server names into channel_names. Entries the
// previous refresh wrote (ownedNames) are dropped first; an entry set by hand
// for the same channel wins over the panel's. It returns the full list to
// write and the part of it that the refresh now owns.
func planNames(existing, ownedNames []string, links []Link) (final, owned []string) {
	kept := dropOwned(existing, ownedNames)
	manual := map[string]struct{}{}
	for _, entry := range kept {
		if id, _, ok := strings.Cut(entry, "="); ok {
			manual[strings.TrimSpace(id)] = struct{}{}
		}
	}
	final = append([]string(nil), kept...)
	seen := map[string]struct{}{}
	for _, l := range links {
		if l.Name == "" {
			continue
		}
		id := channels.ID(l.URI)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := manual[id]; ok {
			continue
		}
		entry := id + "=" + l.Name
		final = append(final, entry)
		owned = append(owned, entry)
	}
	return final, owned
}
