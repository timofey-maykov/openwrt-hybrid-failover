package routing

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/listroutes"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
)

// FlatList is one list together with the section it belongs to. Buttons carry
// the position in FlattenLists, which keeps callback data short.
type FlatList struct {
	Section routesreport.Section
	List    routesreport.List
}

func FlattenLists(secs []routesreport.Section) []FlatList {
	var out []FlatList
	for _, sec := range secs {
		for _, l := range sec.Lists {
			out = append(out, FlatList{Section: sec, List: l})
		}
	}
	return out
}

// ListTitle is the name shown to the user for a list.
func ListTitle(l routesreport.List) string {
	if l.Kind == "user_list" && l.Title != "" {
		return l.Title
	}
	return l.Key
}

// ChannelLabel names a channel id of sec the way the bot shows it.
func ChannelLabel(sec routesreport.Section, id string) string {
	switch id {
	case listroutes.ChannelAuto, "":
		return "пул"
	case listroutes.ChannelBalance:
		return "балансировка"
	case listroutes.ChannelDirect:
		return "напрямую"
	case listroutes.ChannelBlock:
		return "блок"
	}
	for _, c := range sec.Channels {
		if c.ID == id {
			return c.Name
		}
	}
	return "канал удален, идет через пул"
}
