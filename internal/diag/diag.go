package diag

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

type Report struct {
	SingboxRunning bool            `json:"singbox_running"`
	NFTOK          bool            `json:"nft_ok"`
	ClashOK        bool            `json:"clash_ok"`
	FakeIPOK       *bool           `json:"fakeip_ok,omitempty"`
	FakeIPSkipped  bool            `json:"fakeip_skipped,omitempty"`
	ActiveOutbound string          `json:"active_outbound,omitempty"`
	Errors         []string        `json:"errors,omitempty"`
	Failover       *FailoverInfo   `json:"failover,omitempty"`
	Channels       []ChannelStatus `json:"channels,omitempty"`
	Controller     []ControllerSection `json:"controller,omitempty"`
	Meta           MetaInfo        `json:"meta,omitempty"`
	DryRun         []DryRunHint    `json:"dry_run,omitempty"`
}

// ControllerSection is runtime state from the failover policy loop.
type ControllerSection struct {
	Section       string `json:"section"`
	Policy        string `json:"policy"`
	Mode          string `json:"mode"`
	Active        string `json:"active"`
	PrimaryOK     bool   `json:"primary_ok"`
	PrimaryDelay  int    `json:"primary_delay_ms,omitempty"`
	FailStreak    int    `json:"fail_streak"`
	RecoverStreak int    `json:"recover_streak"`
	LastProbeAt   string `json:"last_probe_at,omitempty"`
	LastSwitchAt  string `json:"last_switch_at,omitempty"`
	ActiveSince   string `json:"active_since,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

// DryRunHint describes what the controller would do next (no switch).
type DryRunHint struct {
	Section    string `json:"section"`
	Suggestion string `json:"suggestion"`
}

func GlobalCheck(clashURL, selectorTag string) Report {
	r := Report{}
	singboxRunning := false
	if out, err := exec.Command("pidof", "sing-box").CombinedOutput(); err == nil && len(out) > 0 {
		r.SingboxRunning = true
		singboxRunning = true
	} else {
		r.Errors = append(r.Errors, "sing-box not running")
	}
	if err := netlink.Check(); err != nil {
		r.Errors = append(r.Errors, err.Error())
	} else {
		r.NFTOK = true
	}
	cli := clash.New(clashURL, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if resp, err := cli.Proxies(ctx); err != nil {
		r.Errors = append(r.Errors, "clash api: "+err.Error())
	} else {
		r.ClashOK = true
		if selectorTag != "" {
			if p, ok := resp.Proxies[selectorTag]; ok {
				r.ActiveOutbound = p.Now
			}
		}
	}
	if shouldCheckFakeIP(singboxRunning) {
		if err := CheckFakeIP(); err != nil {
			r.Errors = append(r.Errors, "fakeip: "+err.Error())
			ok := false
			r.FakeIPOK = &ok
		} else {
			ok := true
			r.FakeIPOK = &ok
		}
	} else {
		r.FakeIPSkipped = true
	}
	return r
}

func shouldCheckFakeIP(singboxRunning bool) bool {
	if os.Getenv("HF_CHECK_FAKEIP") == "1" {
		return true
	}
	return singboxRunning
}

func CheckProxy(testURL string) error {
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(testURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

// CheckFakeIP verifies router DNS reaches sing-box fakeip via 127.0.0.42.
func CheckFakeIP() error {
	out, err := exec.Command("dig", "+short", "+time=2", "+tries=1",
		"@"+singbox.DNSInboundAddress, singbox.FAKEIPTestDomain).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dig @%s: %w: %s", singbox.DNSInboundAddress, err, string(out))
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return fmt.Errorf("no fakeip response for %s", singbox.FAKEIPTestDomain)
	}
	if !strings.HasPrefix(result, "198.18.") {
		return fmt.Errorf("unexpected fakeip address %q (want 198.18.x.x)", result)
	}
	if os.Getenv("HF_CHECK_FAKEIP_ROUTE") != "1" {
		return nil
	}
	resolve := fmt.Sprintf("%s:%d:%s", singbox.FAKEIPTestDomain, singbox.FakeIPTestPort, result)
	checkURL := fmt.Sprintf("https://%s:%d/check", singbox.FAKEIPTestDomain, singbox.FakeIPTestPort)
	curlOut, err := exec.Command("curl", "-m", "5", "-sSf", "--resolve", resolve, checkURL).CombinedOutput()
	if err != nil {
		return fmt.Errorf("curl fakeip check: %w: %s", err, string(curlOut))
	}
	return nil
}
