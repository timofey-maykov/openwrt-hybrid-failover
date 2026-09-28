package historywatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
)

type fakeSender struct{ texts []string }

func (f *fakeSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if m, ok := c.(tgbotapi.MessageConfig); ok {
		f.texts = append(f.texts, m.Text)
	}
	return tgbotapi.Message{}, nil
}

func setup(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	stateFile = filepath.Join(dir, "run", "history.state")
	legacyOffsetFile = filepath.Join(dir, "run", "history.offset")
	historyFile = filepath.Join(dir, "history.jsonl")
}

func writeHistory(t *testing.T, events ...notify.Event) {
	t.Helper()
	var b strings.Builder
	for _, ev := range events {
		line, _ := json.Marshal(ev)
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(historyFile, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ev(sec int, to string) notify.Event {
	return notify.Event{Time: time.Date(2026, 9, 29, 12, 0, sec, 0, time.UTC), Section: "glob", To: to}
}

func TestFormatEvent(t *testing.T) {
	s := formatEvent(notify.Event{
		Time:    time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC),
		Section: "glob",
		From:    "glob-awg-out",
		To:      "glob-urltest-out",
		Reason:  "primary outage",
	}, "ax6000")
	if !strings.Contains(s, "ax6000") || !strings.Contains(s, "glob-urltest-out") {
		t.Fatalf("unexpected format: %q", s)
	}
}

func TestTrimLine(t *testing.T) {
	if trimLine("\nfoo\r\n") != "foo" {
		t.Fatal("trim failed")
	}
}

func TestDeliversEachEventOnceAcrossRotation(t *testing.T) {
	setup(t)
	f := &fakeSender{}
	admins := []int64{1}

	writeHistory(t, ev(1, "a"), ev(2, "b"))
	pollOnce(f, admins, "r")
	if len(f.texts) != 2 {
		t.Fatalf("first poll sent %d, want 2", len(f.texts))
	}

	// Rotation drops the oldest line and appends a new one: the file keeps
	// roughly the same size, which broke the byte offset.
	writeHistory(t, ev(2, "b"), ev(3, "c"))
	pollOnce(f, admins, "r")
	if len(f.texts) != 3 || !strings.Contains(f.texts[2], "→ c") {
		t.Fatalf("after rotation sent %v", f.texts)
	}

	pollOnce(f, admins, "r")
	if len(f.texts) != 3 {
		t.Fatalf("repeat poll resent events: %d", len(f.texts))
	}
}

func TestSameTimestampBurst(t *testing.T) {
	setup(t)
	f := &fakeSender{}
	writeHistory(t, ev(1, "a"))
	pollOnce(f, []int64{1}, "")
	writeHistory(t, ev(1, "a"), ev(1, "b"))
	pollOnce(f, []int64{1}, "")
	if len(f.texts) != 2 || !strings.Contains(f.texts[1], "→ b") {
		t.Fatalf("burst handling sent %v", f.texts)
	}
}

func TestUpgradeFromOffsetDoesNotResend(t *testing.T) {
	setup(t)
	f := &fakeSender{}
	_ = os.MkdirAll(filepath.Dir(legacyOffsetFile), 0o755)
	_ = os.WriteFile(legacyOffsetFile, []byte("123"), 0o644)
	writeHistory(t, ev(1, "a"), ev(2, "b"))
	pollOnce(f, []int64{1}, "")
	if len(f.texts) != 0 {
		t.Fatalf("upgrade resent history: %v", f.texts)
	}
	writeHistory(t, ev(1, "a"), ev(2, "b"), ev(3, "c"))
	pollOnce(f, []int64{1}, "")
	if len(f.texts) != 1 {
		t.Fatalf("new event after upgrade: %v", f.texts)
	}
}

func TestPartialTrailingLineWaits(t *testing.T) {
	setup(t)
	f := &fakeSender{}
	line, _ := json.Marshal(ev(1, "a"))
	_ = os.WriteFile(historyFile, line[:len(line)-3], 0o644)
	pollOnce(f, []int64{1}, "")
	if len(f.texts) != 0 {
		t.Fatal("sent a half-written line")
	}
}
