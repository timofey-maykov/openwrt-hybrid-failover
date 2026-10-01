package cmd

import (
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/chanmetrics"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/listroutes"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// runRPCListRoutes reports channels, lists and per-list bindings of every
// vpn/proxy section for the LuCI services tab and the bot.
func runRPCListRoutes() int {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return rpcErr(err.Error())
	}
	snap, _ := engine.ReadRuntimeSnapshot()
	metrics, _ := chanmetrics.Read(chanmetrics.FineFile)
	emitJSON(routesreport.Report{OK: true, Sections: buildRoutesReport(pkg, snap, metrics)})
	return 0
}

// buildRoutesReport assembles the report from UCI, the engine snapshot and the channel metrics.
func buildRoutesReport(pkg *uci.Package, snap engine.RuntimeSnapshot, metrics chanmetrics.File) []routesreport.Section {
	var out []routesreport.Section
	for _, name := range pkg.SectionNamesOrdered("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		typ := sec.Get("connection_type", "")
		if typ != "vpn" && typ != "proxy" {
			continue
		}
		rs := routesreport.Section{Name: name, Type: typ}
		tags := make(map[string]engine.ChannelRuntime)
		for _, ch := range snap.Channels {
			if ch.Section == name {
				tags[ch.ID] = ch
			}
		}
		var sugg []listroutes.SuggestChannel
		for _, ch := range channels.List(sec) {
			rc := routesreport.Channel{ID: ch.ID, Name: ch.Name, Primary: ch.Primary, Host: channelHost(ch.Link)}
			if rt, ok := tags[ch.ID]; ok {
				rc.Tag, rc.Active, rc.Rx, rc.Tx = rt.Tag, rt.Active, rt.Rx, rt.Tx
				if d, ok := snap.Delays[rt.Tag]; ok && (d.OK || d.Error != "") {
					up := d.OK
					rc.Up = &up
					rc.DelayMs = d.DelayMs
				}
			}
			if t, ok := metrics.Totals[name+"/"+ch.ID]; ok && rc.Tag == "" {
				rc.Active, rc.Rx, rc.Tx = t.Active, t.Rx, t.Tx
			}
			rs.Channels = append(rs.Channels, rc)
			sugg = append(sugg, listroutes.SuggestChannel{ID: ch.ID, DelayMs: rc.DelayMs, Down: rc.Up != nil && !*rc.Up})
		}
		routes := listroutes.Routes(pkg, name)
		bound := listroutes.BoundLists(routes)
		var keys []string
		addList := func(key, title, kind string) {
			l := routesreport.List{Key: key, Title: title, Kind: kind, Weight: listroutes.Weight(key), Channel: listroutes.ChannelAuto}
			if r, ok := bound[key]; ok {
				l.Route, l.Channel = r.Name, r.Channel
			}
			rs.Lists = append(rs.Lists, l)
			keys = append(keys, key)
		}
		for _, svc := range sec.GetList("community_lists") {
			if svc = strings.TrimSpace(svc); svc != "" {
				addList(svc, svc, "community")
			}
		}
		if sec.Get("user_domain_list_type", "disabled") != "disabled" || sec.Get("user_subnet_list_type", "disabled") != "disabled" {
			addList(listroutes.KeySectionUser, "", "user")
		}
		if len(sec.GetList("local_domain_lists")) > 0 {
			addList(listroutes.KeySectionLocal, "", "local")
		}
		for _, ul := range listroutes.UserLists(pkg, name) {
			addList(ul.Key(), ul.Title, "user_list")
		}
		known := make(map[string]bool, len(keys))
		for _, k := range keys {
			known[k] = true
		}
		chIDs := make(map[string]bool)
		for _, c := range rs.Channels {
			chIDs[c.ID] = true
		}
		live := make(map[string]engine.BindingRuntime)
		for _, b := range snap.Bindings {
			if b.Section == name {
				live[b.Name] = b
			}
		}
		for _, r := range routes {
			b := routesreport.Binding{Name: r.Name, Lists: r.Lists, Channel: r.Channel, OnDown: r.OnDown}
			if lb, ok := live[r.Name]; ok {
				b.Current, b.Via, b.Missing = lb.Current, lb.Via, lb.Missing
			}
			switch r.Channel {
			case listroutes.ChannelAuto, listroutes.ChannelBalance, listroutes.ChannelDirect, listroutes.ChannelBlock:
			default:
				if !chIDs[r.Channel] {
					b.Missing = true
					rs.Warnings = append(rs.Warnings, "missing_channel:"+r.Name)
				}
			}
			for _, l := range r.Lists {
				if !known[l] {
					rs.Warnings = append(rs.Warnings, "unknown_list:"+r.Name+":"+l)
				}
			}
			rs.Bindings = append(rs.Bindings, b)
		}
		for i := range rs.Channels {
			for _, l := range rs.Lists {
				if l.Channel == rs.Channels[i].ID {
					rs.Channels[i].Weight += l.Weight
				}
			}
		}
		if len(rs.Channels) < 2 && len(routes) > 0 {
			rs.Warnings = append(rs.Warnings, "single_channel")
		}
		rs.Suggest = listroutes.Suggest(keys, sugg)
		out = append(out, rs)
	}
	return out
}

func channelHost(link string) string {
	name := channels.DefaultName(link)
	if i := strings.LastIndex(name, " "); i >= 0 {
		return name[i+1:]
	}
	return ""
}
