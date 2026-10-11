package watchdog

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Release checks: once per UpdateInterval the watchdog asks core whether a
// newer release exists and offers it once per version. The first check waits
// a little after start so a bot restart does not hit GitHub right away.
const updateFirstDelay = 3 * time.Minute

type updateState struct {
	checked     time.Time
	notifiedTag string
}

func (w *Watchdog) checkUpdates(ctx context.Context, t *target) {
	w.mu.Lock()
	interval, started := w.cfg.UpdateInterval, w.started
	last := t.upd.checked
	now := w.now()
	w.mu.Unlock()
	if interval <= 0 || now.Sub(started) < updateFirstDelay {
		return
	}
	if !last.IsZero() && now.Sub(last) < interval {
		return
	}
	w.mu.Lock()
	t.upd.checked = now
	w.mu.Unlock()

	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	res, err := t.Router.CheckUpdate(cctx)
	cancel()
	if err != nil {
		w.log.Warn("update check failed", "router", t.ID, "err", err)
		return
	}
	if !res.Available || res.Tag == "" {
		return
	}
	w.mu.Lock()
	seen := t.upd.notifiedTag == res.Tag
	if !seen {
		t.upd.notifiedTag = res.Tag
	}
	name, id := t.Name, t.ID
	w.mu.Unlock()
	if seen {
		return
	}
	w.notifyRich(releaseNotice(name, res.Installed, res.Latest, res.Notes), releaseButtons(id, res.URL))
}

func releaseNotice(router, installed, latest, notes string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⬆️ %s: вышла Hybrid Failover %s, установлена %s.", router, latest, installed)
	if n := strings.TrimSpace(notes); n != "" {
		b.WriteString("\n\n" + trimNotes(n, 700))
	}
	b.WriteString("\n\nОбновление ставит пакеты, затем перезапускает сервис и бота. Интернет прервётся на несколько секунд.")
	return b.String()
}

func releaseButtons(routerID, url string) [][]Button {
	rows := [][]Button{{{Text: "⬆️ Обновить сейчас", Data: "cmd:/upgrade " + routerID}}}
	if url != "" {
		rows = append(rows, []Button{{Text: "📝 Описание релиза", URL: url}})
	}
	return rows
}

// trimNotes keeps the first lines of release notes within n runes.
func trimNotes(notes string, n int) string {
	r := []rune(notes)
	if len(r) <= n {
		return notes
	}
	cut := string(r[:n])
	if i := strings.LastIndex(cut, "\n"); i > n/2 {
		cut = cut[:i]
	}
	return cut + "\n…"
}
