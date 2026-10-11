package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UpdateCheck is what core RPC UpdateCheck reports about releases.
type UpdateCheck struct {
	Installed   string `json:"installed"`
	Latest      string `json:"latest"`
	Tag         string `json:"tag"`
	Available   bool   `json:"available"`
	PublishedAt string `json:"published_at,omitempty"`
	Notes       string `json:"notes,omitempty"`
	URL         string `json:"url,omitempty"`
	CheckedAt   int64  `json:"checked_at"`
	Error       string `json:"error,omitempty"`
}

// UpdateState is the progress of a running or finished update.
type UpdateState struct {
	State      string   `json:"state"` // idle, running, done, failed
	Step       string   `json:"step,omitempty"`
	Message    string   `json:"message,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	Packages   []string `json:"packages,omitempty"`
	StartedAt  int64    `json:"started_at,omitempty"`
	FinishedAt int64    `json:"finished_at,omitempty"`
}

// UpdateStatus is RPC UpdateStatus: the installed version, the last check
// and the state of the last update.
type UpdateStatus struct {
	Installed string       `json:"installed"`
	State     UpdateState  `json:"state"`
	Check     *UpdateCheck `json:"check,omitempty"`
}

const updateCheckTimeout = 2 * time.Minute

// CheckUpdate asks GitHub through core for the latest release.
func (s Service) CheckUpdate(ctx context.Context) (UpdateCheck, error) {
	ctx, cancel := withTimeout(ctx, updateCheckTimeout)
	defer cancel()
	out, err := s.runner.RunCoreRPC(ctx, "UpdateCheck")
	var res UpdateCheck
	// Core exits 1 when the check itself failed but still prints the result.
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		if err != nil {
			return res, err
		}
		return res, fmt.Errorf("update check: %w", jerr)
	}
	if res.Error != "" {
		return res, fmt.Errorf("%s", res.Error)
	}
	return res, nil
}

// ApplyUpdate starts the update in the background on the router. Core
// restarts itself and the bot when it is done, so the answer comes from the
// new version.
func (s Service) ApplyUpdate(ctx context.Context) error {
	out, err := s.runner.RunCoreRPC(ctx, "UpdateApply")
	if err != nil {
		return err
	}
	var res struct {
		Started bool   `json:"started"`
		Error   string `json:"error,omitempty"`
	}
	if json.Unmarshal([]byte(out), &res) == nil && !res.Started && res.Error != "" {
		return fmt.Errorf("%s", res.Error)
	}
	return nil
}

func (s Service) UpdateStatus(ctx context.Context) (UpdateStatus, error) {
	out, err := s.runner.RunCoreRPC(ctx, "UpdateStatus")
	if err != nil {
		return UpdateStatus{}, err
	}
	var st UpdateStatus
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		return UpdateStatus{}, fmt.Errorf("update status: %w", err)
	}
	return st, nil
}
