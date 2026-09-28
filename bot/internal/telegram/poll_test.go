package telegram

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestSplitMessageShortUnchanged(t *testing.T) {
	if got := splitMessage("привет"); len(got) != 1 || got[0] != "привет" {
		t.Fatalf("got %v", got)
	}
}

func TestSplitMessageKeepsLinesAndLimit(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "hybrid-failover.glob.failover_proxy_links='vless://строка-%03d'\n", i)
	}
	text := strings.TrimRight(b.String(), "\n")
	parts := splitMessage(text)
	if len(parts) < 2 {
		t.Fatalf("expected several parts, got %d", len(parts))
	}
	for _, p := range parts {
		if n := utf8.RuneCountInString(p); n > maxMessageRunes {
			t.Fatalf("part too long: %d", n)
		}
	}
	if strings.Join(parts, "\n") != text {
		t.Fatal("content changed after split")
	}
}

func TestSplitMessageHugeLine(t *testing.T) {
	line := strings.Repeat("ж", maxMessageRunes*2+10)
	parts := splitMessage(line)
	if len(parts) != 3 || strings.Join(parts, "") != line {
		t.Fatalf("huge line split into %d parts", len(parts))
	}
}

func TestIsConflict(t *testing.T) {
	if !isConflict(&tgbotapi.Error{Code: 409, Message: "Conflict: terminated by other getUpdates request"}) {
		t.Fatal("409 not detected")
	}
	if isConflict(&tgbotapi.Error{Code: 401, Message: "Unauthorized"}) {
		t.Fatal("401 treated as conflict")
	}
	if isConflict(errors.New("dial tcp: timeout")) {
		t.Fatal("network error treated as conflict")
	}
}

func TestBackoffCapped(t *testing.T) {
	if backoff(1).Seconds() != 3 || backoff(20).Minutes() != 1 {
		t.Fatalf("backoff %v %v", backoff(1), backoff(20))
	}
}
