package routerctl

import (
	"fmt"
	"strings"
)

func fmtDuration(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	d := sec / 86400
	h := sec % 86400 / 3600
	m := sec % 3600 / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dд %dч %dм", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dч %dм", h, m)
	case m > 0:
		return fmt.Sprintf("%dм", m)
	default:
		return fmt.Sprintf("%dс", sec)
	}
}

func fmtMB(bytes float64) string {
	return fmt.Sprintf("%.0f МБ", bytes/1024/1024)
}

func fmtKB(kb float64) string {
	switch {
	case kb >= 1024*1024:
		return fmt.Sprintf("%.1f ГБ", kb/1024/1024)
	case kb >= 1024:
		return fmt.Sprintf("%.0f МБ", kb/1024)
	default:
		return fmt.Sprintf("%.0f КБ", kb)
	}
}

// tailLines keeps the last n lines.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// clip shortens text to roughly max runes, cutting at a line end.
func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := string(r[len(r)-max:])
	if i := strings.Index(cut, "\n"); i >= 0 && i < len(cut)-1 {
		cut = cut[i+1:]
	}
	return "…\n" + cut
}

// small JSON helpers over map[string]any from encoding/json
func sub(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func list(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

func str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return ""
}

func num(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func flag(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}
