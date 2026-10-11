package subscription

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

// Status is what the last refresh learned, kept for the bot and LuCI. URLs in
// it are masked; it holds no secret.
type Status struct {
	UpdatedAt time.Time `json:"updated_at"`
	// OK is false when the refresh failed before writing any links.
	OK      bool     `json:"ok"`
	Error   string   `json:"error,omitempty"`
	Sources []Source `json:"sources"`
}

// WriteStatus records the outcome of a refresh.
func WriteStatus(st Status) error {
	if err := os.MkdirAll(filepath.Dir(paths.SubscriptionStatus), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := paths.SubscriptionStatus + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, paths.SubscriptionStatus)
}

// ReadStatus returns the recorded outcome; ok is false when there is none yet.
func ReadStatus() (Status, bool) {
	raw, err := os.ReadFile(paths.SubscriptionStatus)
	if err != nil {
		return Status{}, false
	}
	var st Status
	if json.Unmarshal(raw, &st) != nil {
		return Status{}, false
	}
	return st, true
}
