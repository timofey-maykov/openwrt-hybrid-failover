package historywatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

const offsetFile = "/var/run/hybrid-failover-bot/history.offset"

// Run polls failover history and notifies Telegram admins on new events.
func Run(ctx context.Context, api *tgbotapi.BotAPI, adminIDs []int64, interval time.Duration) {
	if api == nil || len(adminIDs) == 0 || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	pollOnce(api, adminIDs)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollOnce(api, adminIDs)
		}
	}
}

func pollOnce(api *tgbotapi.BotAPI, adminIDs []int64) {
	data, err := os.ReadFile(paths.HistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		return
	}
	offset := readOffset()
	if offset > int64(len(data)) {
		offset = 0
	}
	chunk := data[offset:]
	if len(chunk) == 0 {
		return
	}
	lines := splitLines(string(chunk))
	for _, line := range lines {
		line = trimLine(line)
		if line == "" {
			continue
		}
		var ev notify.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		text := formatEvent(ev)
		for _, id := range adminIDs {
			msg := tgbotapi.NewMessage(id, text)
			_, _ = api.Send(msg)
		}
	}
	_ = writeOffset(offset + int64(len(chunk)))
}

func formatEvent(ev notify.Event) string {
	when := ev.Time.Format(time.RFC3339)
	if ev.Time.IsZero() {
		when = "?"
	}
	reason := ev.Reason
	if reason == "" {
		reason = "-"
	}
	return fmt.Sprintf("Failover [%s]\n%s → %s\n%s\n(%s)", ev.Section, ev.From, ev.To, reason, when)
}

func readOffset() int64 {
	b, err := os.ReadFile(offsetFile)
	if err != nil {
		return 0
	}
	var n int64
	fmt.Sscanf(string(b), "%d", &n)
	return n
}

func writeOffset(n int64) error {
	_ = os.MkdirAll(filepath.Dir(offsetFile), 0o755)
	return os.WriteFile(offsetFile, []byte(fmt.Sprintf("%d", n)), 0o644)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimLine(s string) string {
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
