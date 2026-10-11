package watchdog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
)

// Subscription notices: a refresh that failed, a subscription about to end
// or already over. The state is re-read every tick (it is a file core
// writes), a notice goes out when the situation changes and again once a day
// while it lasts.
const (
	subscriptionSoonDays = 3
	subscriptionRenotify = 24 * time.Hour
)

type subscriptionState struct {
	key      string // what was last reported, "" when all is well
	notified time.Time
}

// subscriptionReport is what the watchdog has to say about subscriptions;
// key identifies the situation so the same one is not repeated every tick.
type subscriptionReport struct {
	key  string
	text string
	link string // where to renew or ask for help
}

// evaluateSubscriptions turns the status into a report, nil when fine.
func evaluateSubscriptions(st routing.SubscriptionStatus, now time.Time) *subscriptionReport {
	if st.Configured == 0 {
		return nil
	}
	var parts, keys []string
	link := st.PartnerURL
	for i, src := range st.Sources {
		name := src.Title
		if name == "" {
			name = fmt.Sprintf("подписка %d", i+1)
		}
		if src.SupportURL != "" && link == st.PartnerURL {
			link = src.SupportURL
		}
		if src.Expire == 0 {
			continue
		}
		left := time.Unix(src.Expire, 0).Sub(now)
		switch {
		case left <= 0:
			parts = append(parts, fmt.Sprintf("⛔ %s закончилась %s", name, time.Unix(src.Expire, 0).Local().Format("02.01.2006")))
			keys = append(keys, fmt.Sprintf("expired:%d:%d", i, src.Expire))
		case left <= subscriptionSoonDays*24*time.Hour:
			days := int(left.Hours() / 24)
			parts = append(parts, fmt.Sprintf("⏳ %s заканчивается %s", name, inDays(days)))
			keys = append(keys, fmt.Sprintf("soon:%d:%d:%d", i, src.Expire, days))
		}
	}
	if st.Error != "" {
		parts = append(parts, "⚠ Подписки не обновились: "+oneLine(st.Error, 160))
		keys = append(keys, "error:"+oneLine(st.Error, 80))
	}
	if len(parts) == 0 {
		return nil
	}
	return &subscriptionReport{key: strings.Join(keys, "|"), text: strings.Join(parts, "\n"), link: link}
}

func inDays(d int) string {
	switch {
	case d <= 0:
		return "сегодня"
	case d == 1:
		return "завтра"
	default:
		return fmt.Sprintf("через %d дн.", d)
	}
}

func (w *Watchdog) checkSubscriptions(ctx context.Context, t *target) {
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	st, err := t.Router.SubscriptionStatus(cctx)
	cancel()
	if err != nil {
		return // older core without the RPC, or the router is unreachable: the health check covers that
	}
	now := w.now()
	rep := evaluateSubscriptions(st, now)

	w.mu.Lock()
	if rep == nil {
		t.subs = subscriptionState{}
		w.mu.Unlock()
		return
	}
	due := t.subs.key != rep.key || now.Sub(t.subs.notified) >= subscriptionRenotify
	if !due {
		w.mu.Unlock()
		return
	}
	t.subs = subscriptionState{key: rep.key, notified: now}
	name := t.Name
	w.mu.Unlock()

	text := "📰 " + name + "\n" + rep.text
	var rows [][]Button
	if rep.link != "" {
		rows = append(rows, []Button{{Text: "💳 Продлить или спросить", URL: rep.link}})
	}
	rows = append(rows, []Button{{Text: "🔄 Обновить подписки", Data: "cmd:/subscription_refresh"}, {Text: "📰 Подписки", Data: "nav:subs"}})
	w.notifyRich(text, rows)
}
