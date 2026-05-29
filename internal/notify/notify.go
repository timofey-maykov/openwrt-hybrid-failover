package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Event struct {
	Time     time.Time `json:"time"`
	Section  string    `json:"section"`
	From     string    `json:"from,omitempty"`
	To       string    `json:"to"`
	Reason   string    `json:"reason,omitempty"`
}

func RecordFailover(section, from, to, reason string) error {
	ev := Event{
		Time:    time.Now().UTC(),
		Section: section,
		From:    from,
		To:      to,
		Reason:  reason,
	}
	return appendHistory(ev)
}

func appendHistory(ev Event) error {
	_ = rotateHistoryIfNeeded()
	dir := filepath.Dir(paths.HistoryFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(paths.HistoryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func ReadHistory(limit int) ([]Event, error) {
	data, err := os.ReadFile(paths.HistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var events []Event
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

func rotateHistoryIfNeeded() error {
	maxLines := historyMaxLines()
	data, err := os.ReadFile(paths.HistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := splitLines(string(data))
	if len(lines) <= maxLines {
		return nil
	}
	trim := lines[len(lines)-maxLines:]
	var buf strings.Builder
	for _, line := range trim {
		if line == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return os.WriteFile(paths.HistoryFile, []byte(buf.String()), 0o644)
}

func historyMaxLines() int {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return 500
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return 500
	}
	v := strings.TrimSpace(settings.Get("history_max_lines", ""))
	if v == "" {
		return 500
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 50 {
		return 500
	}
	if n > 10000 {
		return 10000
	}
	return n
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

// SendWebhook POSTs JSON event to configured URL (no-op if url empty).
func SendWebhook(url string, ev Event) error {
	if url == "" {
		return nil
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: unexpected status %s", resp.Status)
	}
	return nil
}
