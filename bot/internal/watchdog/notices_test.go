package watchdog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
)

// richRig captures notices with their buttons.
type richRig struct {
	*rig
	rows [][][]Button
}

func newRichRig(t *testing.T, cfg Config) *richRig {
	g := &richRig{rig: newRig(t, cfg)}
	g.w.SetRichNotifier(func(text string, rows [][]Button) {
		g.notesM.Lock()
		g.notes = append(g.notes, text)
		g.rows = append(g.rows, rows)
		g.notesM.Unlock()
	})
	return g
}

func (g *richRig) lastButtons() []Button {
	g.notesM.Lock()
	defer g.notesM.Unlock()
	if len(g.rows) == 0 {
		return nil
	}
	var out []Button
	for _, r := range g.rows[len(g.rows)-1] {
		out = append(out, r...)
	}
	return out
}

func subStatus(expireIn time.Duration, now time.Time, err string) routing.SubscriptionStatus {
	st := routing.SubscriptionStatus{Configured: 1, Interval: "12h", PartnerURL: "https://t.me/partner_bot", Refreshed: true, Error: err,
		Sources: []routing.SubscriptionSource{{URL: "https://p/…abcd", Title: "PulseVPN 3", SupportURL: "https://t.me/partner_bot?start=support", Servers: 5}}}
	if expireIn != 0 {
		st.Sources[0].Expire = now.Add(expireIn).Unix()
	}
	return st
}

func TestSubscriptionFarFromExpiryIsSilent(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.sub = subStatus(300*24*time.Hour, g.clock, "")
	g.ticks(10)
	if g.count() != 0 {
		t.Fatalf("spoke: %v", g.notes)
	}
}

func TestSubscriptionEndingSoonIsReportedOncePerDay(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.sub = subStatus(2*24*time.Hour+time.Hour, g.clock, "")
	g.tick()
	g.said("📰 OpenWrt", "PulseVPN 3 заканчивается через 2 дн.")
	bs := g.lastButtons()
	if len(bs) < 2 || bs[0].URL != "https://t.me/partner_bot?start=support" || bs[1].Data != "cmd:/subscription_refresh" {
		t.Fatalf("buttons: %+v", bs)
	}
	n := g.count()
	g.ticks(20) // 10 minutes: nothing new
	if g.count() != n {
		t.Fatalf("repeated within a day: %v", g.notes)
	}
	g.clock = g.clock.Add(25 * time.Hour)
	g.r.sub = subStatus(24*time.Hour+time.Hour, g.clock, "")
	g.tick()
	if g.count() != n+1 {
		t.Fatalf("no daily reminder: %d notices", g.count())
	}
	g.said("заканчивается завтра")
}

func TestExpiredSubscriptionAndFailedRefresh(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.sub = subStatus(-time.Hour, g.clock, "https://p/…abcd: HTTP 403: Подписка не активна")
	g.tick()
	all := g.said("⛔ PulseVPN 3 закончилась", "⚠ Подписки не обновились: https://p/…abcd: HTTP 403: Подписка не активна")
	if strings.Contains(all, "SECRET") {
		t.Fatal("leak")
	}
	g.r.sub = subStatus(200*24*time.Hour, g.clock, "")
	g.tick()
	g.tick()
	if g.count() != 1 {
		t.Fatalf("a renewed subscription must go quiet, got %v", g.notes)
	}
	// The same situation after a day is reported again.
	g.r.sub = subStatus(-time.Hour, g.clock, "")
	g.tick()
	if g.count() != 2 {
		t.Fatalf("expired again must be reported: %d", g.count())
	}
}

func TestOldCoreWithoutSubscriptionRPCIsIgnored(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.subErr = errors.New(`unknown rpc method "SubscriptionStatus"`)
	g.ticks(5)
	if g.count() != 0 {
		t.Fatalf("spoke: %v", g.notes)
	}
}

func TestEvaluateSubscriptionsNothingConfigured(t *testing.T) {
	if rep := evaluateSubscriptions(routing.SubscriptionStatus{Error: "x"}, time.Now()); rep != nil {
		t.Fatal("nothing configured must not alarm")
	}
}

func updCfg() Config {
	c := cfg()
	c.UpdateInterval = 24 * time.Hour
	return c
}

func TestReleaseIsOfferedOncePerVersionWithAnInstallButton(t *testing.T) {
	g := newRichRig(t, updCfg())
	g.r.upd = routing.UpdateCheck{Installed: "1.7.64", Latest: "1.7.65", Tag: "v1.7.65", Available: true, Notes: "- bot: release notices\n- core: names from subscriptions", URL: "https://github.com/x/y/releases/tag/v1.7.65"}
	g.ticks(2) // within the first minutes after start: no check yet
	if g.r.updateChecks != 0 {
		t.Fatal("checked right after start")
	}
	g.clock = g.clock.Add(updateFirstDelay)
	g.tick()
	if g.r.updateChecks != 1 {
		t.Fatalf("checks=%d", g.r.updateChecks)
	}
	g.said("⬆️ OpenWrt: вышла Hybrid Failover 1.7.65, установлена 1.7.64", "bot: release notices", "перезапускает сервис и бота")
	bs := g.lastButtons()
	if len(bs) != 2 || bs[0].Data != "cmd:/upgrade r1" || bs[1].URL != "https://github.com/x/y/releases/tag/v1.7.65" {
		t.Fatalf("buttons: %+v", bs)
	}
	n := g.count()
	g.clock = g.clock.Add(25 * time.Hour)
	g.tick()
	if g.r.updateChecks != 2 || g.count() != n {
		t.Fatalf("same version offered twice: checks=%d notices=%d", g.r.updateChecks, g.count())
	}
	g.r.upd.Latest, g.r.upd.Tag = "1.7.66", "v1.7.66"
	g.clock = g.clock.Add(25 * time.Hour)
	g.tick()
	if g.count() != n+1 {
		t.Fatal("a newer version must be offered")
	}
}

func TestReleaseCheckOffAndErrors(t *testing.T) {
	g := newRichRig(t, cfg()) // UpdateInterval 0
	g.r.upd = routing.UpdateCheck{Available: true, Tag: "v9"}
	g.clock = g.clock.Add(time.Hour)
	g.ticks(3)
	if g.r.updateChecks != 0 || g.count() != 0 {
		t.Fatal("checked although turned off")
	}
	g2 := newRichRig(t, updCfg())
	g2.r.updErr = errors.New("github: timeout")
	g2.clock = g2.clock.Add(updateFirstDelay)
	g2.tick()
	if g2.count() != 0 {
		t.Fatalf("an error must not reach the chat: %v", g2.notes)
	}
	if _, err := g2.r.CheckUpdate(context.Background()); err == nil {
		t.Fatal("test setup")
	}
}

func TestTrimNotes(t *testing.T) {
	long := strings.Repeat("строка новостей\n", 100)
	got := trimNotes(long, 100)
	if len([]rune(got)) > 103 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
	if got := trimNotes("short", 100); got != "short" {
		t.Fatal(got)
	}
}

func TestNetworkFailureOnAnActiveSubscriptionHasNoRenewButton(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.sub = subStatus(300*24*time.Hour, g.clock, `Get "https://p/…abcd": dial tcp: lookup p on 192.168.11.1:53: i/o timeout`)
	g.tick()
	g.said("⚠ Подписки не обновились", "Подписка активна до", "серверы из прошлого обновления продолжают работать")
	for _, b := range g.lastButtons() {
		if b.URL != "" {
			t.Fatalf("renew button on a network failure: %+v", g.lastButtons())
		}
	}
	// Another network failure with different wording is the same situation.
	n := g.count()
	g.r.sub = subStatus(300*24*time.Hour, g.clock, `Get "https://p/…abcd": read: connection reset by peer`)
	g.tick()
	if g.count() != n {
		t.Fatalf("repeated for a reworded network error: %v", g.notes)
	}
}

func TestPanelRefusalKeepsTheRenewButton(t *testing.T) {
	g := newRichRig(t, cfg())
	g.r.sub = subStatus(300*24*time.Hour, g.clock, "https://p/…abcd: HTTP 403: Подписка не активна")
	g.tick()
	bs := g.lastButtons()
	if len(bs) == 0 || bs[0].URL == "" {
		t.Fatalf("panel refusal must offer renewing: %+v", bs)
	}
	all := g.said("HTTP 403")
	if strings.Contains(all, "продолжают работать") {
		t.Fatal("must not reassure when the panel refused the subscription")
	}
}
