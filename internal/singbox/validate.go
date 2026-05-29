package singbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// CheckOutboundJSON validates raw outbound JSON with sing-box check on a minimal config.
func CheckOutboundJSON(raw, tag string) error {
	var ob map[string]any
	if err := json.Unmarshal([]byte(raw), &ob); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if _, err := exec.LookPath("sing-box"); err != nil {
		return nil
	}

	ob["tag"] = tag
	cfg := NewConfig()
	cfg.AddOutbound(map[string]any{"type": "direct", "tag": DirectTag})
	cfg.AddOutbound(ob)
	cfg.DNS = map[string]any{
		"servers": []map[string]any{
			{"tag": BootstrapTag, "type": "udp", "server": DefaultBootstrapDNS, "server_port": 53},
		},
		"rules": []any{},
		"final": BootstrapTag,
	}
	cfg.Route = map[string]any{
		"rules": []any{},
		"final": DirectTag,
	}

	data, err := cfg.JSON()
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "hybrid-failover-outbound-*.json")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	out, err := exec.Command("sing-box", "check", "-c", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check: %w: %s", err, string(out))
	}
	return nil
}
