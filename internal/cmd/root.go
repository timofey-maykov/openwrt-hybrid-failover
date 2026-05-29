package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lifecycle"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/lists"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/migrate"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/netlink"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/notify"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/pending"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subscription"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}
	switch args[0] {
	case "migrate":
		return runMigrate(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "apply":
		return runApply(args[1:])
	case "start":
		return runStart(args[1:])
	case "stop":
		return runStop(args[1:])
	case "reload":
		return runReload(args[1:])
	case "restart":
		if runStop(nil) != 0 {
			return 1
		}
		return runStart(nil)
	case "status":
		return runStatus(args[1:])
	case "rpc":
		return runRPC(args[1:])
	case "pending":
		return runPending(args[1:])
	case "check", "check-nft":
		return runCheckNFT(args[1:])
	case "check-proxy":
		return runCheckProxy(args[1:])
	case "check-fakeip":
		return runCheckFakeIP(args[1:])
	case "global-check":
		return runGlobalCheck(args[1:])
	case "list-update":
		return runListUpdate(args[1:])
	case "subscription-refresh":
		return runSubscriptionRefresh(args[1:])
	case "monitor":
		return runMonitor(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `hybrid-failover: Hybrid Failover Core

Usage:
  hybrid-failover migrate [--dry-run]
  hybrid-failover validate [--dry-run]
  hybrid-failover apply [--dry-run]
  hybrid-failover start|stop|reload|restart|status|monitor
  hybrid-failover rpc <method> [json args]
  hybrid-failover pending capture|validate|apply|rollback
  hybrid-failover check-nft|check-proxy|check-fakeip|global-check
  hybrid-failover list-update|subscription-refresh

`)
}

func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "")
	_ = fs.Parse(args)
	changed, err := migrate.Run(paths.UCIConfig, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if changed {
		fmt.Println("migration applied")
	} else {
		fmt.Println("migration: no changes")
	}
	return 0
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "")
	uciPath := fs.String("uci", paths.UCIConfig, "")
	_ = fs.Parse(args)

	pkg, err := uci.Load(*uciPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !pkg.HasOutboundSection() {
		fmt.Fprintln(os.Stderr, "no outbound section")
		return 1
	}
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		ci := sec.Get("urltest_check_interval", "")
		idle := sec.Get("urltest_idle_timeout", "")
		if err := validation.ValidateURLTestDurationPair(ci, idle); err != nil {
			fmt.Fprintln(os.Stderr, name+":", err)
			return 1
		}
		for _, link := range append(sec.GetList("failover_proxy_links"), sec.GetList("urltest_proxy_links")...) {
			if err := validation.ValidateProxyURI(link); err != nil {
				fmt.Fprintln(os.Stderr, name+":", err)
				return 1
			}
		}
	}
	_, err = lifecycle.Apply(lifecycle.Options{UCIPath: *uciPath, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("validate: ok")
	return 0
}

func runApply(args []string) int {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "")
	uciPath := fs.String("uci", paths.UCIConfig, "")
	_ = fs.Parse(args)
	res, err := lifecycle.Apply(lifecycle.Options{UCIPath: *uciPath, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("apply: hash=%s changed=%v\n", res.ConfigHash, res.Changed)
	return 0
}

func runStart(args []string) int {
	_ = args
	res, err := lifecycle.StartPipeline(lifecycle.StartOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("started (config hash %s)\n", res.ConfigHash)
	return 0
}

func runStop(args []string) int {
	_ = args
	lifecycle.CancelBackground()
	lists.RemoveCron()
	lists.ClearPID()
	_ = dnsmasq.Restore()
	_ = netlink.Teardown()
	_ = execSingbox("stop")
	fmt.Println("stopped")
	return 0
}

func runReload(args []string) int {
	_ = args
	res, err := lifecycle.ApplyAndReloadIfChanged(lifecycle.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := lifecycle.RefreshPerClient(""); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("reloaded changed=%v\n", res.Changed)
	return 0
}

func runMonitor(args []string) int {
	_ = args
	fmt.Println("hybrid-failover monitor: policy controller + watchdog")
	lifecycle.StartBackground(paths.UCIConfig)
	select {}
}

func runStatus(args []string) int {
	_ = args
	report := buildStatusReport(false)
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	return 0
}

func runHealth(args []string) int {
	_ = args
	report := buildStatusReport(true)
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	return 0
}

func statusContext() (clashURL, mainSection string, sec *uci.Section) {
	mainSection = paths.DefaultMainSection
	clashURL = "http://" + clash.DetectListenAddress("")
	if pkg, err := uci.Load(paths.UCIConfig); err == nil {
		if settings := pkg.Section("settings"); settings != nil {
			clashURL = clash.ResolveBaseURL(settings)
			if ms := settings.Get("main_section", ""); ms != "" {
				mainSection = ms
			}
		}
		sec = pkg.Section(mainSection)
	}
	return clashURL, mainSection, sec
}

func runRPC(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: hybrid-failover rpc <method> [json]")
		return 2
	}
	method := args[0]
	rest := args[1:]
	switch method {
	case "Validate", "validate":
		return runValidate(nil)
	case "Apply", "apply":
		return runApply(nil)
	case "Status", "status":
		return runStatus(nil)
	case "Health", "health":
		return runHealth(nil)
	case "PendingValidate", "pending_validate":
		return runPending([]string{"validate"})
	case "PendingApply", "pending_apply":
		return runPending([]string{"apply"})
	case "PendingRollback", "pending_rollback":
		return runPending([]string{"rollback"})
	case "CapturePending", "pending_capture":
		return runPending([]string{"capture"})
	case "History", "history":
		events, err := notify.ReadHistory(50)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		enc, _ := json.MarshalIndent(events, "", "  ")
		fmt.Println(string(enc))
		return 0
	case "CheckNft", "check_nft":
		return runRPCCheckNFT()
	case "CheckFakeip", "check_fakeip":
		return runRPCCheckFakeIP()
	case "GlobalCheck", "global_check":
		return runRPCGlobalCheck()
	case "DecodeURI", "decode_uri":
		return runRPCDecodeURI(rest)
	case "SwitchProxy", "switch_proxy":
		return runRPCSwitchProxy(rest)
	case "ExportHistory", "export_history":
		return runRPCExportHistory(rest)
	case "BackupUCI", "backup_uci":
		return runRPCBackupUCI()
	case "RestoreUCI", "restore_uci":
		return runRPCRestoreUCI(rest)
	case "DuplicateSection", "duplicate_section":
		return runRPCDuplicateSection(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown rpc method %q\n", method)
		return 2
	}
}

func runPending(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: pending capture|validate|apply|rollback")
		return 2
	}
	store := pending.NewStore("")
	switch args[0] {
	case "capture":
		if err := pending.CaptureFromUCIChanges(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("pending: captured")
	case "validate":
		if err := store.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("pending: ok")
	case "apply":
		if err := store.ApplyViaUCI(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("pending: applied")
	case "rollback":
		if err := store.Rollback(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("pending: rolled back")
	default:
		return 2
	}
	return 0
}

func runCheckNFT(args []string) int {
	_ = args
	if err := netlink.Check(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("nft: ok")
	return 0
}

func runCheckProxy(args []string) int {
	url := "https://www.gstatic.com/generate_204"
	if len(args) > 0 {
		url = args[0]
	}
	if err := diag.CheckProxy(url); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("proxy check: ok")
	return 0
}

func runCheckFakeIP(args []string) int {
	_ = args
	if err := diag.CheckFakeIP(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("fakeip check: ok")
	return 0
}

func runGlobalCheck(args []string) int {
	_ = args
	return runStatus(nil)
}

func runListUpdate(args []string) int {
	_ = args
	u := lists.NewUpdater(false)
	if err := u.UpdateOnce(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := lifecycle.ApplyAndReloadIfChanged(lifecycle.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("list-update: ok changed=%v\n", res.Changed)
	return 0
}

func runSubscriptionRefresh(args []string) int {
	fs := flag.NewFlagSet("subscription-refresh", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "")
	merge := fs.Bool("merge", true, "")
	uciPath := fs.String("uci", paths.UCIConfig, "")
	section := fs.String("section", paths.DefaultMainSection, "")
	_ = fs.Parse(args)
	pkg, err := uci.Load(*uciPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	settings := pkg.Section("settings")
	if settings == nil {
		fmt.Fprintln(os.Stderr, "no settings section")
		return 1
	}
	mainSection := *section
	if override := settings.Get("main_section", ""); override != "" {
		mainSection = override
	}
	urls := settings.GetList("subscription_urls")
	if len(urls) == 0 {
		fmt.Println("subscription: no urls configured")
		return 0
	}
	f := subscription.NewFetcher()
	links, err := f.FetchURLs(urls)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("subscription: fetched %d links for section %q\n", len(links), mainSection)
	if *dryRun {
		fmt.Println("subscription: dry-run, UCI not modified")
		return 0
	}
	listKey, err := subscription.ApplyToUCI(*uciPath, mainSection, links, *merge)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("subscription: wrote %d links to %s.%s\n", len(links), mainSection, listKey)
	if _, err := lifecycle.Apply(lifecycle.Options{UCIPath: *uciPath}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := lifecycle.ReloadSingbox(""); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("subscription: applied and reloaded")
	return 0
}

func execSingbox(action string) error {
	out, err := exec.Command(paths.SingboxInit, action).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box %s: %w: %s", action, err, string(out))
	}
	return nil
}
