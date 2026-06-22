package ipc

import (
	"os"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func TestSubmitSwitchRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths.SwitchRequestFile = dir + "/switch-request.json"
	paths.SwitchResponseFile = dir + "/switch-response.json"

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			ProcessPendingSwitch(func(section, outbound string) (string, error) {
				if section != "glob" || outbound != "glob-2-out" {
					t.Errorf("unexpected switch %q -> %q", section, outbound)
				}
				return "glob-urltest-out", nil
			})
			time.Sleep(20 * time.Millisecond)
		}
	}()

	from, to, err := SubmitSwitch("glob", "glob-2-out", 2*time.Second)
	close(done)
	if err != nil {
		t.Fatalf("SubmitSwitch: %v", err)
	}
	if from != "glob-urltest-out" || to != "glob-2-out" {
		t.Fatalf("got from=%q to=%q", from, to)
	}
	_ = os.Remove(paths.SwitchRequestFile)
	_ = os.Remove(paths.SwitchResponseFile)
}
