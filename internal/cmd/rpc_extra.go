package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
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
	report = diag.EnrichReport(report, clashURL, mainSection, sec)
	schema := ""
	if pkg, err := uci.Load(paths.UCIConfig); err == nil {
		if settings := pkg.Section("settings"); settings != nil {
			schema = settings.Get("config_schema_version", "")
		}
	}
	report.Meta = diag.BuildMeta(schema)
	states, _ := failover.ReadRuntimeState()
	if len(report.Controller) == 0 && len(states) > 0 {
		report.Controller = diag.MapControllerStates(states)
	}
	for _, h := range failover.BuildDryRunHints(states) {
		report.DryRun = append(report.DryRun, diag.DryRunHint{Section: h.Section, Suggestion: h.Suggestion})
	}
	if health {
		report = diag.ProbeChannels(report, clashURL, mainSection, sec)
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
	ok := report.SingboxRunning && report.NFTOK && report.ClashOK
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
	clashURL, _, _ := statusContext()
	cli := clash.New(clashURL, 12*time.Second)
	selector := singbox.OutboundTag(section)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	prev, _ := cli.ActiveOutbound(ctx, selector)
	if err := cli.SwitchProxy(ctx, selector, outbound); err != nil {
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
