package telegram

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestHTTPClientDropsADeadConnection(t *testing.T) {
	hang := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-hang }))
	defer srv.Close()
	defer close(hang)

	c := NewHTTPClient(150 * time.Millisecond)
	start := time.Now()
	resp, err := c.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("a request nobody answers must time out")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("gave up after %v", time.Since(start))
	}
	if HTTPTimeout <= time.Duration(pollTimeoutSeconds)*time.Second {
		t.Fatal("client timeout must exceed the long poll or every idle poll fails")
	}
	tr := c.Transport.(*http.Transport)
	if tr.TLSNextProto == nil || len(tr.TLSNextProto) != 0 {
		t.Fatal("HTTP/2 must stay disabled")
	}
	if !tr.DisableKeepAlives {
		t.Fatal("a connection that died during a router reload must never be reused")
	}
}
