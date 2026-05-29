package historywatch

import (
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
)

func TestFormatEvent(t *testing.T) {
	s := formatEvent(notify.Event{
		Time:    time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC),
		Section: "glob",
		From:    "glob-awg-out",
		To:      "glob-urltest-out",
		Reason:  "primary outage",
	})
	if s == "" {
		t.Fatal("empty format")
	}
}

func TestTrimLine(t *testing.T) {
	if trimLine("\nfoo\r\n") != "foo" {
		t.Fatal("trim failed")
	}
}
