package diag

import (
	"os"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/version"
)

// MetaInfo is build/runtime metadata for status RPC.
type MetaInfo struct {
	CoreVersion    string `json:"core_version,omitempty"`
	SingboxVersion string `json:"singbox_version,omitempty"`
	UCISchema      string `json:"uci_schema_version,omitempty"`
}

func BuildMeta(schemaVersion string) MetaInfo {
	return MetaInfo{
		CoreVersion: version.Core,
		UCISchema:   schemaVersion,
	}
}

// WritePrometheusTextfile exports minimal metrics for node_exporter textfile collector.
func WritePrometheusTextfile(r Report, path string) error {
	if path == "" {
		return nil
	}
	var b strings.Builder
	up := 0
	if r.SingboxRunning && r.NFTOK && r.ClashOK {
		up = 1
	}
	b.WriteString("# HELP hybrid_failover_up Routing stack healthy.\n")
	b.WriteString("# TYPE hybrid_failover_up gauge\n")
	b.WriteString("hybrid_failover_up ")
	b.WriteString(itoa(up))
	b.WriteByte('\n')
	if r.ActiveOutbound != "" {
		b.WriteString("# HELP hybrid_failover_active_outbound_info Active outbound tag.\n")
		b.WriteString("# TYPE hybrid_failover_active_outbound_info gauge\n")
		b.WriteString("hybrid_failover_active_outbound_info{outbound=\"")
		b.WriteString(escapeLabel(r.ActiveOutbound))
		b.WriteString("\"} 1\n")
	}
	for _, ch := range r.Channels {
		b.WriteString("hybrid_failover_channel_delay_ms{name=\"")
		b.WriteString(escapeLabel(ch.Name))
		b.WriteString("\",available=\"")
		if ch.Available {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString("\"} ")
		b.WriteString(itoa(ch.DelayMs))
		b.WriteByte('\n')
	}
	for _, cs := range r.Controller {
		if cs.Section == "" {
			continue
		}
		b.WriteString("hybrid_failover_fail_streak{section=\"")
		b.WriteString(escapeLabel(cs.Section))
		b.WriteString("\"} ")
		b.WriteString(itoa(cs.FailStreak))
		b.WriteByte('\n')
		b.WriteString("hybrid_failover_recover_streak{section=\"")
		b.WriteString(escapeLabel(cs.Section))
		b.WriteString("\"} ")
		b.WriteString(itoa(cs.RecoverStreak))
		b.WriteByte('\n')
		probeVal := 0
		if cs.PrimaryOK {
			probeVal = 1
		}
		b.WriteString("hybrid_failover_probe_ok{section=\"")
		b.WriteString(escapeLabel(cs.Section))
		b.WriteString("\"} ")
		b.WriteString(itoa(probeVal))
		b.WriteByte('\n')
		if cs.PrimaryDelay > 0 {
			b.WriteString("hybrid_failover_primary_delay_ms{section=\"")
			b.WriteString(escapeLabel(cs.Section))
			b.WriteString("\"} ")
			b.WriteString(itoa(cs.PrimaryDelay))
			b.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func escapeLabel(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
