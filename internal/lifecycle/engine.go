package lifecycle

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const enginePlanHashPath = "/var/run/hybrid-failover/engine-plan.sha256"
const engineSyncRequestPath = "/var/run/hybrid-failover/engine-sync.request"

var (
	engineSyncMu        sync.Mutex
	lastEnginePlanHash  string
	lastUCIConfigFP     string
)

func uciConfigFingerprint(path string) (string, error) {
	if path == "" {
		path = paths.UCIConfig
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", st.ModTime().UnixNano(), st.Size()), nil
}

func initEnginePlanHashFromDisk() {
	data, err := os.ReadFile(enginePlanHashPath)
	if err != nil {
		return
	}
	engineSyncMu.Lock()
	lastEnginePlanHash = strings.TrimSpace(string(data))
	engineSyncMu.Unlock()
}

// RequestEngineSync asks the monitor process to reload the engine plan on the next sync tick.
func RequestEngineSync() error {
	if err := os.MkdirAll("/var/run/hybrid-failover", 0o755); err != nil {
		return err
	}
	return os.WriteFile(engineSyncRequestPath, []byte("1\n"), 0o644)
}

func consumeEngineSyncRequest() bool {
	if _, err := os.Stat(engineSyncRequestPath); err != nil {
		return false
	}
	_ = os.Remove(engineSyncRequestPath)
	return true
}

// NoteEnginePlanHash records the plan hash already applied in the monitor process.
func NoteEnginePlanHash(hash string) {
	engineSyncMu.Lock()
	lastEnginePlanHash = hash
	engineSyncMu.Unlock()
}

// SyncNativeEnginePlan applies UCI/plan changes to the running monitor engine without a manual restart.
func SyncNativeEnginePlan(ctx context.Context, uciPath string) {
	_ = ctx
	engineSyncMu.Lock()
	defer engineSyncMu.Unlock()

	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	if consumeEngineSyncRequest() {
		lastUCIConfigFP = ""
		lastEnginePlanHash = ""
	}
	fp, err := uciConfigFingerprint(uciPath)
	if err == nil && fp == lastUCIConfigFP {
		return
	}

	_, hash, err := compileEnginePlan(uciPath)
	if err != nil {
		log.Printf("hybrid-failover engine sync: compile: %v", err)
		return
	}
	if fp != "" {
		lastUCIConfigFP = fp
	}
	if hash == "" || hash == lastEnginePlanHash {
		return
	}

	if !engine.Default().Running() {
		lastEnginePlanHash = hash
		_ = os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644)
		return
	}

	lastEnginePlanHash = hash
	_ = os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644)
	engine.Default().Stop()
}

// RestartNativeEngine stops the native engine so the monitor loop starts it again from UCI.
func RestartNativeEngine(ctx context.Context, uciPath string) error {
	_ = ctx
	engineSyncMu.Lock()
	defer engineSyncMu.Unlock()

	p, hash, err := compileEnginePlan(uciPath)
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	lastEnginePlanHash = hash
	_ = os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644)
	engine.Default().Stop()
	return nil
}

func compileEnginePlan(uciPath string) (*plan.Plan, string, error) {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return nil, "", err
	}
	if !engine.NativeEnabled(pkg) {
		return nil, "", nil
	}
	p, err := engine.CompilePlan(pkg)
	if err != nil {
		return nil, "", err
	}
	if err := engine.ValidatePlan(p); err != nil {
		return nil, "", err
	}
	return p, plan.Hash(p), nil
}

func applyNativeEngine(uciPath string) (Result, error) {
	p, hash, err := compileEnginePlan(uciPath)
	if err != nil {
		return Result{}, err
	}
	if p == nil {
		return Result{}, nil
	}
	oldHash, _ := os.ReadFile(enginePlanHashPath)
	changed := string(oldHash) != hash+"\n"
	if err := os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644); err != nil {
		return Result{}, err
	}
	if err := engine.Default().ApplyPlan(p); err != nil {
		return Result{}, err
	}
	return Result{ConfigHash: hash, Changed: changed}, nil
}

func reloadNativeEngine(ctx context.Context, uciPath string) error {
	p, hash, err := compileEnginePlan(uciPath)
	if err != nil {
		return err
	}
	if hash == "" {
		return nil
	}
	if err := os.WriteFile(enginePlanHashPath, []byte(hash+"\n"), 0o644); err != nil {
		return err
	}
	// Monitor process picks up hash/plan via SyncNativeEnginePlan.
	if engine.Default().Running() {
		return engine.Default().Reload(ctx, p)
	}
	return nil
}

func stopNativeEngine() {
	engine.Default().Stop()
}

// StopNativeEngine stops the native proxy engine.
func StopNativeEngine() {
	stopNativeEngine()
}
