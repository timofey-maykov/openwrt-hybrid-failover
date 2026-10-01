package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

// RuntimeSnapshot is persisted for status RPC in a separate process.
type RuntimeSnapshot struct {
	UpdatedAt time.Time                    `json:"updated_at"`
	Sections  map[string]SectionRuntime    `json:"sections,omitempty"`
	Delays    map[string]DelayChannelState `json:"delays,omitempty"`
	Channels  []ChannelRuntime             `json:"channels,omitempty"`
	Bindings  []BindingRuntime             `json:"bindings,omitempty"`
}

// ChannelRuntime is one channel of a section with its load since start.
type ChannelRuntime struct {
	Section string `json:"section"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Iface   string `json:"iface,omitempty"`
	Conns   uint64 `json:"conns"`
	Active  int64  `json:"active"`
	Rx      uint64 `json:"rx"`
	Tx      uint64 `json:"tx"`
}

// BindingRuntime is one list_route and where its traffic goes now.
type BindingRuntime struct {
	Name    string   `json:"name"`
	Section string   `json:"section"`
	Lists   []string `json:"lists"`
	Channel string   `json:"channel"`
	OnDown  string   `json:"on_down"`
	Missing bool     `json:"missing,omitempty"`
	// Current is the outbound tag new connections use; Via says how: the
	// bound channel, the pool, direct, balance or block.
	Current string `json:"current,omitempty"`
	Via     string `json:"via"`
}

type SectionRuntime struct {
	URLTestMember string `json:"urltest_member,omitempty"`
}

type DelayChannelState struct {
	DelayMs int    `json:"delay_ms,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
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
			snap.Delays[tag] = DelayChannelState{DelayMs: ms, OK: d.OK && ms > 0, Error: d.Error}
		}
	}
	if rt == nil || p == nil {
		return snap
	}
	snap.Channels = channelRuntime(p, rt.Traffic())
	snap.Bindings = bindingRuntime(p, rt.BindingActive)
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

func channelRuntime(p *plan.Plan, traffic map[string]outbound.TrafficStat) []ChannelRuntime {
	// Only tunnel interfaces carry one channel's bytes. A proxy outbound
	// may also have BindIface (the WAN it leaves through), shared by every
	// proxy and all direct traffic, so its bytes come from the engine.
	ifaces := make(map[string]string, len(p.Outbounds))
	for _, ob := range p.Outbounds {
		if ob.BindIface != "" && (ob.Kind == plan.OutboundDirectBind || ob.Kind == plan.OutboundAWG2Bind) {
			ifaces[ob.Tag] = ob.BindIface
		}
	}
	out := make([]ChannelRuntime, 0, len(p.Channels))
	for _, ch := range p.Channels {
		st := traffic[ch.Tag]
		cr := ChannelRuntime{
			Section: ch.Section, ID: ch.ID, Name: ch.Name, Tag: ch.Tag,
			Iface: ifaces[ch.Tag], Conns: st.Conns, Active: st.Active, Rx: st.Rx, Tx: st.Tx,
		}
		if cr.Iface != "" {
			// Interface-bound connections are spliced in the kernel; the
			// interface counters carry their bytes.
			cr.Rx = ifaceCounter(cr.Iface, "rx_bytes")
			cr.Tx = ifaceCounter(cr.Iface, "tx_bytes")
		}
		out = append(out, cr)
	}
	return out
}

func ifaceCounter(iface, name string) uint64 {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "statistics", name))
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return v
}

func bindingRuntime(p *plan.Plan, active func(tag string) string) []BindingRuntime {
	out := make([]BindingRuntime, 0, len(p.Bindings))
	for _, b := range p.Bindings {
		br := BindingRuntime{
			Name: b.Name, Section: b.Section, Lists: b.Lists,
			Channel: b.Channel, OnDown: b.OnDown, Missing: b.Missing,
		}
		switch {
		case b.Missing:
			br.Current = plan.OutboundTag(b.Section)
			br.Via = "pool"
		case b.Channel == "block":
			br.Via = "block"
		case b.Channel == "direct":
			br.Current = plan.DirectTag
			br.Via = "direct"
		case b.Channel == "balance":
			br.Current = b.OutboundTag
			br.Via = "balance"
		default:
			br.Current = active(b.OutboundTag)
			switch br.Current {
			case b.ChannelTag:
				br.Via = "channel"
			case plan.DirectTag:
				br.Via = "direct"
			default:
				br.Via = "pool"
			}
		}
		out = append(out, br)
	}
	return out
}
