package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

// RuntimeSnapshot is persisted for status RPC in a separate process.
type RuntimeSnapshot struct {
	UpdatedAt time.Time                    `json:"updated_at"`
	Sections  map[string]SectionRuntime    `json:"sections,omitempty"`
	Delays    map[string]DelayChannelState `json:"delays,omitempty"`
}

type SectionRuntime struct {
	URLTestMember string `json:"urltest_member,omitempty"`
}

type DelayChannelState struct {
	DelayMs int  `json:"delay_ms,omitempty"`
	OK      bool `json:"ok"`
}

// Snapshot exports live urltest member selection and delay samples.
func (e *Engine) Snapshot() RuntimeSnapshot {
	snap := RuntimeSnapshot{
		UpdatedAt: time.Now().UTC(),
		Sections:  make(map[string]SectionRuntime),
		Delays:    make(map[string]DelayChannelState),
	}
	e.mu.RLock()
	rt := e.rt
	ctrl := e.ctrl
	p := e.plan
	e.mu.RUnlock()
	if ctrl != nil {
		for tag, d := range ctrl.AllDelays() {
			ms := int(d.Delay.Milliseconds())
			snap.Delays[tag] = DelayChannelState{DelayMs: ms, OK: d.OK && ms > 0}
		}
	}
	if rt == nil || p == nil {
		return snap
	}
	for _, sec := range p.Sections {
		if sec.SelectorTag == "" {
			continue
		}
		member := rt.URLTestActive(sec.Name)
		if member == "" {
			continue
		}
		snap.Sections[sec.Name] = SectionRuntime{URLTestMember: member}
	}
	return snap
}

// WriteRuntimeSnapshot persists Snapshot to the runtime state file.
func WriteRuntimeSnapshot(snap RuntimeSnapshot) error {
	dir := filepath.Dir(paths.EngineRuntimeFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp := paths.EngineRuntimeFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, paths.EngineRuntimeFile)
}

// ReadRuntimeSnapshot loads the last snapshot written by the monitor process.
func ReadRuntimeSnapshot() (RuntimeSnapshot, error) {
	data, err := os.ReadFile(paths.EngineRuntimeFile)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	var snap RuntimeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return RuntimeSnapshot{}, err
	}
	return snap, nil
}

// URLTestMemberFromSnapshot returns the active urltest member for a routing section.
func URLTestMemberFromSnapshot(section string) string {
	snap, err := ReadRuntimeSnapshot()
	if err != nil {
		return ""
	}
	if st, ok := snap.Sections[section]; ok {
		return st.URLTestMember
	}
	return ""
}

// DelaysFromSnapshot returns delay samples keyed by outbound tag.
func DelaysFromSnapshot() map[string]DelayChannelState {
	snap, err := ReadRuntimeSnapshot()
	if err != nil {
		return nil
	}
	return snap.Delays
}
