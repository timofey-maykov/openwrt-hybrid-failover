package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/listroutes"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
)

// ListRoutes reads channels, lists and bindings of the router's sections.
func (s Service) ListRoutes(ctx context.Context) ([]routesreport.Section, error) {
	out, err := s.runner.RunCoreRPC(ctx, "ListRoutes")
	if err != nil {
		return nil, err
	}
	var rep routesreport.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		return nil, fmt.Errorf("list_routes: %w", err)
	}
	return rep.Sections, nil
}

var routeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// RouteSectionName is the list_route the LuCI tab and the bot use for one list.
func RouteSectionName(section, listKey string) string {
	return routeNameRe.ReplaceAllString("lr_"+section+"_"+listKey, "_")
}

// ResolveChannel turns what the user typed (number from /routes, id, name
// prefix or a keyword) into a channel value.
func ResolveChannel(sec routesreport.Section, arg string) (string, error) {
	a := strings.ToLower(strings.TrimSpace(arg))
	switch a {
	case "auto", "pool", "пул":
		return listroutes.ChannelAuto, nil
	case "balance", "баланс":
		return listroutes.ChannelBalance, nil
	case "direct", "напрямую":
		return listroutes.ChannelDirect, nil
	case "block", "блок":
		return listroutes.ChannelBlock, nil
	}
	if n, err := strconv.Atoi(a); err == nil && n >= 1 && n <= len(sec.Channels) {
		return sec.Channels[n-1].ID, nil
	}
	var hits []string
	for _, c := range sec.Channels {
		if c.ID == a {
			return c.ID, nil
		}
		if strings.HasPrefix(strings.ToLower(c.Name), a) {
			hits = append(hits, c.ID)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) > 1 {
		return "", fmt.Errorf("под «%s» подходит несколько каналов, укажите номер из /routes", arg)
	}
	return "", fmt.Errorf("канал «%s» не найден, номера каналов в /routes", arg)
}

// SetListRoute binds one list of section to a channel (pending, like other
// UCI edits: /param_apply applies it). channel auto removes the binding.
func (s Service) SetListRoute(ctx context.Context, section, listKey, channel, onDown string) error {
	name := RouteSectionName(section, listKey)
	key := s.uciPackage + "." + name
	if channel == listroutes.ChannelAuto {
		_, _ = s.runner.Run(ctx, "/sbin/uci", "-q", "delete", key)
		return s.capturePending(ctx)
	}
	cmds := []string{
		fmt.Sprintf("uci set %s=list_route", key),
		fmt.Sprintf("uci set %s.section=%s", key, shellQuote(section)),
		fmt.Sprintf("uci -q delete %s.lists", key),
		fmt.Sprintf("uci add_list %s.lists=%s", key, shellQuote(listKey)),
		fmt.Sprintf("uci set %s.channel=%s", key, shellQuote(channel)),
		fmt.Sprintf("uci set %s.on_down=%s", key, shellQuote(listroutes.NormalizeOnDown(onDown))),
	}
	if _, err := s.runner.Run(ctx, "/bin/sh", "-lc", strings.Join(cmds, " && ")); err != nil {
		return err
	}
	return s.capturePending(ctx)
}

// FormatRoutes renders the report for Telegram.
func FormatRoutes(secs []routesreport.Section) string {
	if len(secs) == 0 {
		return "Нет секций vpn или proxy."
	}
	var b strings.Builder
	for _, sec := range secs {
		fmt.Fprintf(&b, "Секция %s\nКаналы:\n", sec.Name)
		names := map[string]string{}
		for i, c := range sec.Channels {
			names[c.ID] = c.Name
			mark := "⚪"
			if c.Up != nil {
				mark = "❌"
				if *c.Up {
					mark = "✅"
				}
			}
			line := fmt.Sprintf("%d. %s %s", i+1, mark, c.Name)
			if c.DelayMs > 0 {
				line += fmt.Sprintf(" %d мс", c.DelayMs)
			}
			if c.Active > 0 {
				line += fmt.Sprintf(", %d соед.", c.Active)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("Списки:\n")
		live := map[string]routesreport.Binding{}
		for _, bd := range sec.Bindings {
			for _, l := range bd.Lists {
				if _, ok := live[l]; !ok {
					live[l] = bd
				}
			}
		}
		for _, l := range sec.Lists {
			title := l.Key
			if l.Kind == "user_list" && l.Title != "" {
				title = l.Title + " (" + l.Key + ")"
			}
			target := "пул"
			switch l.Channel {
			case listroutes.ChannelAuto, "":
			case listroutes.ChannelBalance:
				target = "балансировка"
			case listroutes.ChannelDirect:
				target = "напрямую"
			case listroutes.ChannelBlock:
				target = "блок"
			default:
				target = names[l.Channel]
				if target == "" {
					target = "канал удален, идет через пул"
				}
			}
			line := "• " + title + " → " + target
			if bd, ok := live[l.Key]; ok && l.Channel != listroutes.ChannelAuto {
				switch {
				case bd.Missing:
				case bd.Via == "pool":
					line += " (канал упал, сейчас через пул)"
				case bd.Via == "direct" && l.Channel != listroutes.ChannelDirect:
					line += " (канал упал, сейчас напрямую)"
				}
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Привязать: /route <список> <номер канала|pool|balance|direct|block> [pool|direct|block]\nПример: /route youtube 2")
	return b.String()
}
