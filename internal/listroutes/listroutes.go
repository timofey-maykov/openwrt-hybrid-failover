// Package listroutes reads the per-list channel bindings of routing sections:
//
//	config user_list 'work'
//		option section 'main'
//		option title 'Работа'
//		option domains_text 'jira.example.com gitlab.example.com'
//		option subnets_text '10.20.0.0/16'
//
//	config list_route 'video'
//		option section 'main'
//		list lists 'youtube'
//		list lists 'user:work'
//		option channel 'a1b2c3d4'   # channel id, auto, balance, direct, block
//		option on_down 'pool'       # pool, direct, block
//
// A list that no binding names stays with its section as before: the
// section's selector (urltest pool) carries it.
package listroutes

import (
	"fmt"
	"net"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// Channel values that are not a channel id.
const (
	ChannelAuto    = "auto"    // the section pool, same as no binding
	ChannelBalance = "balance" // spread connections over all live channels
	ChannelDirect  = "direct"
	ChannelBlock   = "block"
)

// on_down values: what a list bound to one channel does when that channel is down.
const (
	DownPool   = "pool"
	DownDirect = "direct"
	DownBlock  = "block"
)

// List keys, as used in `list lists`.
const (
	KeySectionUser  = "user"  // the section's own user_domains / user_subnets
	KeySectionLocal = "local" // the section's local_domain_lists files
	UserListPrefix  = "user:" // user:<user_list section name>
)

// UserList is a named list of the user's own domains and subnets.
type UserList struct {
	Name    string // UCI section name
	Section string // routing section it belongs to
	Title   string
	Domains []string
	Subnets []string
}

// Key is how bindings refer to the list.
func (u UserList) Key() string { return UserListPrefix + u.Name }

// Route is one binding.
type Route struct {
	Name    string // UCI section name
	Section string
	Lists   []string
	Channel string
	OnDown  string
}

// Bound reports whether the route takes its lists away from the section pool.
func (r Route) Bound() bool {
	return r.Channel != "" && r.Channel != ChannelAuto && len(r.Lists) > 0
}

// UserLists returns the enabled user lists of section, in config order.
func UserLists(pkg *uci.Package, section string) []UserList {
	if pkg == nil {
		return nil
	}
	var out []UserList
	for _, name := range pkg.SectionNamesOrdered("user_list") {
		s := pkg.Section(name)
		if s == nil || !s.GetBool("enabled", true) || s.Get("section", "") != section {
			continue
		}
		ul := UserList{
			Name:    name,
			Section: section,
			Title:   s.Get("title", name),
			Domains: splitItems(s.Get("domains_text", "")),
			Subnets: splitItems(s.Get("subnets_text", "")),
		}
		ul.Domains = append(ul.Domains, cleanList(s.GetList("domains"))...)
		raw := append(ul.Subnets, cleanList(s.GetList("subnets"))...)
		ul.Subnets = nil
		for _, v := range raw {
			if c, ok := NormalizeCIDR(v); ok {
				ul.Subnets = append(ul.Subnets, c)
			}
		}
		if len(ul.Domains) == 0 && len(ul.Subnets) == 0 {
			continue
		}
		out = append(out, ul)
	}
	return out
}

// Routes returns the enabled bindings of section, in config order.
func Routes(pkg *uci.Package, section string) []Route {
	if pkg == nil {
		return nil
	}
	var out []Route
	for _, name := range pkg.SectionNamesOrdered("list_route") {
		s := pkg.Section(name)
		if s == nil || !s.GetBool("enabled", true) || s.Get("section", "") != section {
			continue
		}
		r := Route{
			Name:    name,
			Section: section,
			Lists:   cleanList(s.GetList("lists")),
			Channel: strings.TrimSpace(s.Get("channel", ChannelAuto)),
			OnDown:  NormalizeOnDown(s.Get("on_down", DownPool)),
		}
		if r.Channel == "" {
			r.Channel = ChannelAuto
		}
		out = append(out, r)
	}
	return out
}

// NormalizeOnDown maps unknown values to the default, pool.
func NormalizeOnDown(v string) string {
	switch strings.TrimSpace(v) {
	case DownDirect:
		return DownDirect
	case DownBlock:
		return DownBlock
	default:
		return DownPool
	}
}

// BoundLists maps each list key of section to the binding that takes it. A list
// named by two bindings goes to the first one.
func BoundLists(routes []Route) map[string]Route {
	out := make(map[string]Route)
	for _, r := range routes {
		if !r.Bound() {
			continue
		}
		for _, l := range r.Lists {
			if _, ok := out[l]; !ok {
				out[l] = r
			}
		}
	}
	return out
}

// HasUserLists reports whether section has any enabled user list; such a
// section routes by lists even with no community or section-level lists.
func HasUserLists(pkg *uci.Package, section string) bool {
	return len(UserLists(pkg, section)) > 0
}

func splitItems(text string) []string {
	f := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\n' || r == '\t' || r == '\r' || r == ';'
	})
	return cleanList(f)
}

func cleanList(in []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || strings.HasPrefix(v, "#") || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// NormalizeCIDR accepts an IPv4 CIDR or a single address (made /32).
func NormalizeCIDR(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !strings.Contains(v, "/") {
		ip := net.ParseIP(v)
		if ip == nil || ip.To4() == nil {
			return "", false
		}
		return ip.To4().String() + "/32", true
	}
	ip, n, err := net.ParseCIDR(v)
	if err != nil || ip.To4() == nil {
		return "", false
	}
	return n.String(), true
}

// Validate reports user lists and bindings that cannot work as written.
func Validate(pkg *uci.Package) []error {
	if pkg == nil {
		return nil
	}
	var errs []error
	sections := make(map[string]bool)
	for _, name := range pkg.SectionNames("section") {
		sections[name] = true
	}
	for _, name := range pkg.SectionNamesOrdered("user_list") {
		s := pkg.Section(name)
		if sec := s.Get("section", ""); !sections[sec] {
			errs = append(errs, fmt.Errorf("user_list %s: секция %q не найдена", name, sec))
		}
		items := append(splitItems(s.Get("subnets_text", "")), cleanList(s.GetList("subnets"))...)
		for _, v := range items {
			if _, ok := NormalizeCIDR(v); !ok {
				errs = append(errs, fmt.Errorf("user_list %s: %q не IPv4 и не CIDR", name, v))
			}
		}
	}
	for _, name := range pkg.SectionNamesOrdered("list_route") {
		s := pkg.Section(name)
		if sec := s.Get("section", ""); !sections[sec] {
			errs = append(errs, fmt.Errorf("list_route %s: секция %q не найдена", name, sec))
		}
		if v := strings.TrimSpace(s.Get("on_down", DownPool)); v != DownPool && v != DownDirect && v != DownBlock {
			errs = append(errs, fmt.Errorf("list_route %s: on_down %q, ожидается pool, direct или block", name, v))
		}
	}
	return errs
}
