package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/clientrules"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/delayhistory"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/ipc"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/failover"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func buildStatusReport(health bool) diag.Report {
	clashURL, mainSection, sec := statusContext()
	selectorTag := singbox.OutboundTag(mainSection)
	report := diag.GlobalCheck(clashURL, selectorTag)
	states, _ := failover.ReadRuntimeState()
	if len(report.Controller) == 0 && len(states) > 0 {
		report.Controller = diag.MapControllerStates(states)
	}
	if report.EngineMode == "native" {
		report = diag.EnrichNativeReport(report, mainSection, sec, states)
	} else {
		report = diag.EnrichReport(report, clashURL, mainSection, sec)
	}
	schema := ""
	if pkg, err := uci.Load(paths.UCIConfig); err == nil {
		if settings := pkg.Section("settings"); settings != nil {
			schema = settings.Get("config_schema_version", "")
		}
	}
	report.Meta = diag.BuildMeta(schema)
	for _, h := range failover.BuildDryRunHints(states) {
		report.DryRun = append(report.DryRun, diag.DryRunHint{Section: h.Section, Suggestion: h.Suggestion})
	}
	if health {
		if report.EngineMode == "native" {
			report = diag.ProbeNativeChannels(report, mainSection, sec)
		} else {
			report = diag.ProbeChannels(report, clashURL, mainSection, sec)
		}
	}
	if states, err := failover.ReadRuntimeState(); err == nil && len(states) > 0 {
		report.Controller = diag.MapControllerStates(states)
	}
	_ = diag.WritePrometheusTextfile(report, paths.MetricsPromFile)
	return report
}

func runRPCCheckNFT() int {
	if err := netlink.Check(); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error()})
		return 1
	}
	emitJSON(map[string]any{"ok": true, "message": "nft: ok"})
	return 0
}

func runRPCCheckFakeIP() int {
	if err := diag.CheckFakeIP(); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error()})
		return 1
	}
	emitJSON(map[string]any{"ok": true, "message": "fakeip: ok"})
	return 0
}

func runRPCGlobalCheck() int {
	report := buildStatusReport(false)
	ok := report.NFTOK && report.ClashOK
	if report.EngineMode == "native" {
		ok = report.NFTOK && report.EngineRunning
	} else {
		ok = report.NFTOK && report.ClashOK && report.SingboxRunning
	}
	emitJSON(map[string]any{"ok": ok, "report": report})
	if !ok {
		return 1
	}
	return 0
}

func runRPCDecodeURI(args []string) int {
	uri := strings.TrimSpace(strings.Join(args, " "))
	if uri == "" {
		return rpcErr("uri required")
	}
	desc, err := amnezia.DescribeLink(uri)
	if err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error()})
		return 1
	}
	emitJSON(map[string]any{"ok": true, "summary": desc})
	return 0
}

func runRPCSwitchProxy(args []string) int {
	section, outbound := "", ""
	if len(args) >= 2 {
		section, outbound = args[0], args[1]
	} else if len(args) == 1 {
		var req struct {
			Section  string `json:"section"`
			Outbound string `json:"outbound"`
		}
		if json.Unmarshal([]byte(args[0]), &req) == nil {
			section, outbound = req.Section, req.Outbound
		}
	}
	if section == "" || outbound == "" {
		return rpcErr("usage: SwitchProxy <section> <outbound>")
	}
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return rpcErr(err.Error())
	}
	if err := failover.ValidateSelectorMember(pkg, section, outbound); err != nil {
		return rpcErr(err.Error())
	}
	selector := singbox.OutboundTag(section)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var prev string
	var errSwitch error
	if engine.NativeEnabled(pkg) {
		if !engine.Alive() {
			return rpcErr("native engine not running")
		}
		prev, _, errSwitch = ipc.SubmitSwitch(section, outbound, 15*time.Second)
	} else {
		clashURL, _, _ := statusContext()
		cli := clash.New(clashURL, 12*time.Second)
		prev, _ = cli.ActiveOutbound(ctx, selector)
		errSwitch = cli.SwitchProxy(ctx, selector, outbound)
	}
	if errSwitch != nil {
		emitJSON(map[string]any{"ok": false, "error": errSwitch.Error()})
		return 1
	}
	if err := failover.NoteManualSwitch(section, outbound); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error()})
		return 1
	}
	_ = notify.RecordFailover(section, prev, outbound, "manual")
	emitJSON(map[string]any{"ok": true, "from": prev, "to": outbound})
	return 0
}

func runRPCExportHistory(args []string) int {
	limit := 100
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &limit)
	}
	events, err := notify.ReadHistory(limit)
	if err != nil {
		return rpcErr(err.Error())
	}
	emitJSON(events)
	return 0
}

func runRPCBackupUCI() int {
	out := "/tmp/hybrid-failover-uci-backup.tar.gz"
	cmd := exec.Command("tar", "-czf", out, "-C", "/etc/config", "hybrid-failover")
	if out2, err := cmd.CombinedOutput(); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error(), "output": string(out2)})
		return 1
	}
	emitJSON(map[string]any{"ok": true, "path": out})
	return 0
}

func runRPCRestoreUCI(args []string) int {
	path := "/tmp/hybrid-failover-uci-backup.tar.gz"
	if len(args) > 0 {
		path = strings.TrimSpace(args[0])
	}
	if err := validateBackupPath(path); err != nil {
		return rpcErr(err.Error())
	}
	cmd := exec.Command("tar", "-xzf", path, "-C", "/etc/config", "hybrid-failover")
	if out, err := cmd.CombinedOutput(); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error(), "output": strings.TrimSpace(string(out))})
		return 1
	}
	emitJSON(map[string]any{"ok": true, "message": "restored from " + path})
	return 0
}

func validateBackupPath(path string) error {
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/tmp/") {
		return fmt.Errorf("backup path must be under /tmp")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup file: %w", err)
	}
	return nil
}

func runRPCDuplicateSection(args []string) int {
	from, to := "", ""
	if len(args) >= 2 {
		from, to = args[0], args[1]
	} else if len(args) == 1 {
		var req struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if json.Unmarshal([]byte(args[0]), &req) == nil {
			from, to = req.From, req.To
		}
	}
	if from == "" || to == "" {
		return rpcErr("usage: DuplicateSection <from> <to>")
	}
	if from == to {
		return rpcErr("from and to must differ")
	}
	if err := duplicateUCISection(from, to); err != nil {
		return rpcErr(err.Error())
	}
	emitJSON(map[string]any{"ok": true, "from": from, "to": to})
	return 0
}

func duplicateUCISection(from, to string) error {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return err
	}
	src := pkg.Section(from)
	if src == nil {
		return fmt.Errorf("section %q not found", from)
	}
	if pkg.Section(to) != nil {
		return fmt.Errorf("section %q already exists", to)
	}
	pkgName := paths.UCIPackage
	if err := uciRun("set", fmt.Sprintf("%s.%s=%s", pkgName, to, src.Type)); err != nil {
		return err
	}
	for k, v := range src.Options {
		if err := uciRun("set", fmt.Sprintf("%s.%s.%s=%s", pkgName, to, k, v)); err != nil {
			return err
		}
	}
	for k, vals := range src.Lists {
		for _, v := range vals {
			if err := uciRun("add_list", fmt.Sprintf("%s.%s.%s=%s", pkgName, to, k, v)); err != nil {
				return err
			}
		}
	}
	return uciRun("commit", pkgName)
}

func uciRun(args ...string) error {
	out, err := exec.Command("uci", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uci %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func emitJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func rpcErr(msg string) int {
	emitJSON(map[string]any{"ok": false, "error": msg})
	return 1
}

func runRPCListClients() int {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return rpcErr(err.Error())
	}
	rules := clientrules.ListRules(pkg)
	if rules == nil {
		rules = []clientrules.Rule{}
	}
	emitJSON(map[string]any{"ok": true, "rules": rules})
	return 0
}

func runRPCDelayHistory() int {
	data, err := delayhistory.ReadAll()
	if err != nil {
		return rpcErr(err.Error())
	}
	emitJSON(map[string]any{"ok": true, "channels": data})
	return 0
}

func runRPCListUpdateRPC() int {
	if code := runListUpdate(nil); code != 0 {
		emitJSON(map[string]any{"ok": false, "error": "list-update failed"})
		return code
	}
	emitJSON(map[string]any{"ok": true, "message": "list-update completed"})
	return 0
}

func runRPCSubscriptionRefreshRPC() int {
	if code := runSubscriptionRefresh(nil); code != 0 {
		emitJSON(map[string]any{"ok": false, "error": "subscription-refresh failed"})
		return code
	}
	emitJSON(map[string]any{"ok": true, "message": "subscription-refresh completed"})
	return 0
}

func runRPCBackupDownload() int {
	out := "/tmp/hybrid-failover-uci-backup.tar.gz"
	cmd := exec.Command("tar", "-czf", out, "-C", "/etc/config", "hybrid-failover")
	if out2, err := cmd.CombinedOutput(); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error(), "output": string(out2)})
		return 1
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return rpcErr(err.Error())
	}
	emitJSON(map[string]any{
		"ok":       true,
		"filename": "hybrid-failover-backup.tar.gz",
		"data":     base64.StdEncoding.EncodeToString(raw),
	})
	return 0
}

func runRPCMetrics() int {
	report := buildStatusReport(false)
	var b strings.Builder
	up := 0
	if report.SingboxRunning && report.NFTOK && report.ClashOK {
		up = 1
	}
	b.WriteString("# HELP hybrid_failover_up Routing stack healthy.\n")
	b.WriteString("# TYPE hybrid_failover_up gauge\n")
	b.WriteString(fmt.Sprintf("hybrid_failover_up %d\n", up))
	for _, cs := range report.Controller {
		if cs.Section == "" {
			continue
		}
		b.WriteString("hybrid_failover_fail_streak{section=\"")
		b.WriteString(escapeProm(cs.Section))
		b.WriteString("\"} ")
		b.WriteString(strconv.Itoa(cs.FailStreak))
		b.WriteByte('\n')
		b.WriteString("hybrid_failover_recover_streak{section=\"")
		b.WriteString(escapeProm(cs.Section))
		b.WriteString("\"} ")
		b.WriteString(strconv.Itoa(cs.RecoverStreak))
		b.WriteByte('\n')
		if cs.PrimaryOK {
			b.WriteString("hybrid_failover_probe_ok{section=\"")
			b.WriteString(escapeProm(cs.Section))
			b.WriteString("\"} 1\n")
		} else {
			b.WriteString("hybrid_failover_probe_ok{section=\"")
			b.WriteString(escapeProm(cs.Section))
			b.WriteString("\"} 0\n")
		}
	}
	emitJSON(map[string]any{"ok": true, "prometheus": b.String()})
	return 0
}

func escapeProm(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}
