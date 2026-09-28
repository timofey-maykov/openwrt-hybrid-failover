package routing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
)

type fakeExec struct {
	rpc   map[string]string
	calls []string
	dl    map[string]time.Duration
}

func (f *fakeExec) Run(ctx context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return "", nil
}

func (f *fakeExec) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	f.calls = append(f.calls, "rpc "+method)
	if dl, ok := ctx.Deadline(); ok {
		if f.dl == nil {
			f.dl = map[string]time.Duration{}
		}
		f.dl[method] = time.Until(dl)
	}
	out, ok := f.rpc[method]
	if !ok {
		return "", errors.New("exit status 1: {\"ok\":false}")
	}
	return out, nil
}

func report() diag.Report {
	return diag.Report{
		EngineMode: "native", EngineRunning: true, NFTOK: true,
		ActiveOutbound: "glob-awg-out",
		Channels: []diag.ChannelStatus{
			{Name: "glob-awg-out", Display: "AWG", Available: true, DelayMs: 42, Selected: true, Probed: true},
			{Name: "glob-1-out", Available: false, Probed: true, Detail: "timeout"},
			{Name: "glob-2-out", Available: false},
		},
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestHealthUsesCoreRPCNotClashAPI(t *testing.T) {
	f := &fakeExec{rpc: map[string]string{"Health": mustJSON(report())}}
	// Clash API points nowhere: native engine has none, the bot must not need it.
	svc := NewService(f, "http://127.0.0.1:1", "/etc/init.d/hybrid-failover", "", "", time.Second)
	chs, err := svc.ChannelHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 3 || !chs[0].Available || chs[0].Detail != "42ms" || !strings.Contains(chs[0].Name, "активный") {
		t.Fatalf("channels: %+v", chs)
	}
	if chs[1].Detail != "timeout" || chs[2].Detail != "не проверялся" {
		t.Fatalf("details: %+v", chs)
	}
	if f.dl["Health"] < 50*time.Second {
		t.Fatalf("health got a short deadline: %v", f.dl["Health"])
	}
}

func TestStatusDoesNotProbeChannels(t *testing.T) {
	f := &fakeExec{rpc: map[string]string{"Status": mustJSON(report())}}
	svc := NewService(f, "", "/etc/init.d/hybrid-failover", "", "", time.Second)
	out, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "routing state: active") || !strings.Contains(out, "active_outbound: glob-awg-out") {
		t.Fatalf("status: %s", out)
	}
	for _, c := range f.calls {
		if c == "rpc Health" {
			t.Fatal("status ran channel probes")
		}
	}
}

func TestPendingApplyDoesNotRestart(t *testing.T) {
	f := &fakeExec{rpc: map[string]string{"PendingValidate": "ok", "PendingApply": "ok"}}
	svc := NewService(f, "", "/etc/init.d/hybrid-failover", "", "", time.Second)
	if err := svc.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "init.d") {
			t.Fatalf("apply restarted the service: %v", f.calls)
		}
	}
	if f.dl["PendingApply"] < 2*time.Minute {
		t.Fatalf("apply deadline too short: %v", f.dl["PendingApply"])
	}
}

func TestFailoverHistoryFormatted(t *testing.T) {
	evs := []notify.Event{
		{Time: time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC), Section: "glob", From: "glob-awg-out", To: "glob-1-out", Reason: "primary outage"},
		{Time: time.Date(2026, 9, 29, 10, 5, 0, 0, time.UTC), Section: "glob", To: "glob-awg-out"},
	}
	f := &fakeExec{rpc: map[string]string{"History": mustJSON(evs)}}
	svc := NewService(f, "", "", "", "", time.Second)
	out, err := svc.FailoverHistory(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "{") || !strings.Contains(out, "- → glob-awg-out") || strings.Contains(out, "outage") {
		t.Fatalf("history: %q", out)
	}
}
