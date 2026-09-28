package historywatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

// State lives next to the old byte-offset file. A byte offset breaks as soon
// as core rotates history.jsonl (it rewrites the file keeping the last N
// lines): events were either skipped or the whole history was resent. The
// time of the last delivered event survives rotation.
var (
	stateFile        = "/var/run/hybrid-failover-bot/history.state"
	legacyOffsetFile = "/var/run/hybrid-failover-bot/history.offset"
	historyFile      = paths.HistoryFile
)

type state struct {
	// Last is the timestamp of the newest event already delivered.
	Last time.Time `json:"last"`
	// SameTime counts delivered events that share Last exactly, so a burst
	// written within one clock tick is neither lost nor repeated.
	SameTime int `json:"same_time"`
}

type sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Run polls failover history and notifies Telegram admins on new events.
func Run(ctx context.Context, api *tgbotapi.BotAPI, adminIDs []int64, interval time.Duration, router string) {
	if api == nil || len(adminIDs) == 0 || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	pollOnce(api, adminIDs, router)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollOnce(api, adminIDs, router)
		}
	}
}

func pollOnce(api sender, adminIDs []int64, router string) {
	events, err := readEvents(historyFile)
	if err != nil || len(events) == 0 {
		return
	}
	st, ok := loadState()
	if !ok {
		st = initialState(events)
	}
	fresh, next := newEvents(events, st)
	for _, ev := range fresh {
		msg := formatEvent(ev, router)
		for _, id := range adminIDs {
			if _, err := api.Send(tgbotapi.NewMessage(id, msg)); err != nil {
				fmt.Fprintf(os.Stderr, "historywatch: send failed for admin %d: %v\n", id, err)
			}
		}
	}
	if len(fresh) > 0 || !ok {
		_ = saveState(next)
	}
}

// initialState decides what counts as already seen when there is no state
// file. After an upgrade from the offset-based watcher everything present was
// delivered already. Otherwise (first install, or /var/run wiped by a reboot
// together with /var/log) the whole current history is new.
func initialState(events []notify.Event) state {
	if _, err := os.Stat(legacyOffsetFile); err == nil {
		_ = os.Remove(legacyOffsetFile)
		_, st := newEvents(events, state{})
		return st
	}
	return state{}
}

// newEvents returns events after st in file order and the state after them.
func newEvents(events []notify.Event, st state) ([]notify.Event, state) {
	var out []notify.Event
	seenAtLast := 0
	for _, ev := range events {
		switch {
		case ev.Time.Before(st.Last):
			continue
		case ev.Time.Equal(st.Last) && !st.Last.IsZero():
			seenAtLast++
			if seenAtLast <= st.SameTime {
				continue
			}
		}
		out = append(out, ev)
	}
	next := st
	for _, ev := range out {
		if ev.Time.After(next.Last) {
			next.Last = ev.Time
			next.SameTime = 1
		} else if ev.Time.Equal(next.Last) {
			next.SameTime++
		}
	}
	return out, next
}

func readEvents(path string) ([]notify.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Ignore a trailing line without newline: core may still be writing it.
	text := string(data)
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		text = text[:i+1]
	} else {
		return nil, nil
	}
	var events []notify.Event
	for _, line := range strings.Split(text, "\n") {
		line = trimLine(line)
		if line == "" {
			continue
		}
		var ev notify.Event
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Time.IsZero() {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func formatEvent(ev notify.Event, router string) string {
	when := ev.Time.Local().Format("2006-01-02 15:04:05")
	if ev.Time.IsZero() {
		when = "?"
	}
	reason := ev.Reason
	if reason == "" {
		reason = "-"
	}
	from := ev.From
	if from == "" {
		from = "-"
	}
	head := "Failover [" + ev.Section + "]"
	if router != "" {
		head = "Failover · " + router + " [" + ev.Section + "]"
	}
	return fmt.Sprintf("%s\n%s → %s\n%s\n(%s)", head, from, ev.To, reason, when)
}

func loadState() (state, bool) {
	b, err := os.ReadFile(stateFile)
	if err != nil {
		return state{}, false
	}
	var st state
	if json.Unmarshal(b, &st) != nil {
		return state{}, false
	}
	return st, true
}

func saveState(st state) error {
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := stateFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, stateFile)
}

func trimLine(s string) string {
	return strings.Trim(s, "\r\n")
}
