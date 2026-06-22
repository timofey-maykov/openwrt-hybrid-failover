package ipc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

type switchRequest struct {
	ID       string `json:"id"`
	Section  string `json:"section"`
	Outbound string `json:"outbound"`
}

type switchResponse struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

// SubmitSwitch queues a selector switch for the monitor process and waits for the result.
func SubmitSwitch(section, outbound string, timeout time.Duration) (from, to string, err error) {
	if section == "" || outbound == "" {
		return "", "", fmt.Errorf("section and outbound required")
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	req := switchRequest{ID: id, Section: section, Outbound: outbound}
	if err := writeJSON(paths.SwitchRequestFile, req); err != nil {
		return "", "", err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, readErr := readResponse(id)
		if readErr == nil {
			_ = os.Remove(paths.SwitchResponseFile)
			if !resp.OK {
				if resp.Error == "" {
					resp.Error = "switch failed"
				}
				return resp.From, resp.To, fmt.Errorf("%s", resp.Error)
			}
			return resp.From, resp.To, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", "", fmt.Errorf("switch request timed out")
}

// ProcessPendingSwitch applies a queued switch request in the monitor process.
func ProcessPendingSwitch(apply func(section, outbound string) (from string, err error)) {
	data, err := os.ReadFile(paths.SwitchRequestFile)
	if err != nil {
		return
	}
	var req switchRequest
	if err := json.Unmarshal(data, &req); err != nil || req.ID == "" {
		_ = os.Remove(paths.SwitchRequestFile)
		return
	}
	_ = os.Remove(paths.SwitchRequestFile)

	resp := switchResponse{ID: req.ID, To: req.Outbound}
	from, err := apply(req.Section, req.Outbound)
	resp.From = from
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.OK = true
	}
	_ = writeJSON(paths.SwitchResponseFile, resp)
}

func readResponse(id string) (switchResponse, error) {
	data, err := os.ReadFile(paths.SwitchResponseFile)
	if err != nil {
		return switchResponse{}, err
	}
	var resp switchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return switchResponse{}, err
	}
	if resp.ID != id {
		return switchResponse{}, fmt.Errorf("waiting")
	}
	return resp, nil
}

func writeJSON(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
