package pending

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/lifecycle"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

// Encoded change ops stored in snapshot values (uci changes replay).
const (
	opAddListPrefix = "@add_list:"
	opDelListPrefix = "@del_list:"
	opDelete        = "@delete"
)

type Store struct {
	dir string
}

type Snapshot struct {
	CreatedAt time.Time         `json:"created_at"`
	Changes   map[string]string `json:"changes"`
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = paths.PendingDir
	}
	return &Store{dir: dir}
}

func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+".json")
}

func (s *Store) Save(changes map[string]string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	snap := Snapshot{CreatedAt: time.Now().UTC(), Changes: changes}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path("pending"), data, 0o600)
}

// SaveChanges merges changes into the pending snapshot (creates one if missing).
func (s *Store) SaveChanges(changes map[string]string) error {
	if len(changes) == 0 {
		return nil
	}
	merged := map[string]string{}
	if snap, err := s.Load(); err == nil {
		for k, v := range snap.Changes {
			merged[k] = v
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for k, v := range changes {
		merged[k] = v
	}
	return s.Save(merged)
}

// SaveChanges is a convenience wrapper for the default pending store directory.
func SaveChanges(changes map[string]string) error {
	return NewStore("").SaveChanges(changes)
}

func (s *Store) Load() (*Snapshot, error) {
	data, err := os.ReadFile(s.path("pending"))
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Store) Validate() error {
	snap, err := s.Load()
	if err != nil {
		return err
	}
	for key, val := range snap.Changes {
		if val == opDelete {
			if err := validation.ValidateUCIKey(key); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(val, opAddListPrefix) || strings.HasPrefix(val, opDelListPrefix) {
			if err := validation.ValidateUCIKey(key); err != nil {
				return err
			}
			raw := val
			if strings.HasPrefix(val, opAddListPrefix) {
				raw = strings.TrimPrefix(val, opAddListPrefix)
			} else {
				raw = strings.TrimPrefix(val, opDelListPrefix)
			}
			if strings.TrimSpace(raw) == "" {
				return fmt.Errorf("empty list value for %s", key)
			}
			continue
		}
		if err := validation.ValidateUCIKey(key); err != nil {
			return err
		}
		norm, err := validation.NormalizeUCIOptionValue(key, val)
		if err != nil {
			return err
		}
		peer := snap.Changes[validation.PeerURLTestUCIKey(key)]
		if err := validation.ValidateURLTestUCISet(key, norm, peer); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Rollback() error {
	return os.Remove(s.path("pending"))
}

// CaptureFromUCIChanges runs `uci changes` for hybrid-failover and merges into the pending store.
func CaptureFromUCIChanges() error {
	out, err := execUCI("changes", paths.UCIPackage)
	if err != nil {
		return fmt.Errorf("uci changes: %w: %s", err, out)
	}
	changes, err := ParseUCIChangesOutput(out)
	if err != nil {
		return err
	}
	return NewStore("").SaveChanges(changes)
}

// ParseUCIChangesOutput parses `uci changes` lines into pending snapshot entries.
func ParseUCIChangesOutput(output string) (map[string]string, error) {
	changes := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, op, err := parseUCIChangeLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse uci change %q: %w", line, err)
		}
		switch op {
		case "delete":
			changes[key] = opDelete
		case "add_list":
			changes[key] = opAddListPrefix + val
		case "del_list":
			changes[key] = opDelListPrefix + val
		default:
			changes[key] = val
		}
	}
	return changes, nil
}

func parseUCIChangeLine(line string) (key, value, op string, err error) {
	if strings.HasPrefix(line, "-") && !strings.Contains(line, "=") {
		return strings.TrimPrefix(line, "-"), "", "delete", nil
	}
	for _, spec := range []struct {
		sep string
		op  string
	}{
		{"+=", "add_list"},
		{"-=", "del_list"},
		{"=", "set"},
	} {
		idx := strings.Index(line, spec.sep)
		if idx < 0 {
			continue
		}
		key = line[:idx]
		raw := line[idx+len(spec.sep):]
		value, err = parseQuotedUCIValue(raw)
		if err != nil {
			return "", "", "", err
		}
		return key, value, spec.op, nil
	}
	return "", "", "", fmt.Errorf("unsupported change syntax")
}

func parseQuotedUCIValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) < 2 || raw[0] != '\'' {
		return raw, nil
	}
	var b strings.Builder
	for i := 1; i < len(raw); i++ {
		if raw[i] == '\'' {
			if i+1 < len(raw) && raw[i+1] == '\'' {
				b.WriteByte('\'')
				i++
				continue
			}
			if i != len(raw)-1 {
				return "", fmt.Errorf("trailing data after quoted value")
			}
			break
		}
		b.WriteByte(raw[i])
	}
	return b.String(), nil
}

func (s *Store) ApplyViaUCI() error {
	snap, err := s.Load()
	if err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return fmt.Errorf("pending validate: %w", err)
	}
	for key, val := range snap.Changes {
		if err := applyPendingChange(key, val); err != nil {
			return err
		}
	}
	if out, err := execUCI("commit", paths.UCIPackage); err != nil {
		return fmt.Errorf("uci commit: %w: %s", err, out)
	}
	if _, err := lifecycle.Apply(lifecycle.Options{}); err != nil {
		return fmt.Errorf("lifecycle apply: %w", err)
	}
	if err := lifecycle.ReloadSingbox(""); err != nil {
		return fmt.Errorf("lifecycle reload: %w", err)
	}
	return os.Remove(s.path("pending"))
}

func applyPendingChange(key, val string) error {
	switch {
	case val == opDelete:
		if out, err := execUCI("delete", key); err != nil {
			return fmt.Errorf("uci delete %s: %w: %s", key, err, out)
		}
	case strings.HasPrefix(val, opAddListPrefix):
		v := strings.TrimPrefix(val, opAddListPrefix)
		if out, err := execUCI("add_list", key+"="+v); err != nil {
			return fmt.Errorf("uci add_list %s: %w: %s", key, err, out)
		}
	case strings.HasPrefix(val, opDelListPrefix):
		v := strings.TrimPrefix(val, opDelListPrefix)
		if out, err := execUCI("del_list", key+"="+v); err != nil {
			return fmt.Errorf("uci del_list %s: %w: %s", key, err, out)
		}
	default:
		norm, _ := validation.NormalizeUCIOptionValue(key, val)
		if out, err := execUCI("set", key+"="+norm); err != nil {
			return fmt.Errorf("uci set %s: %w: %s", key, err, out)
		}
	}
	return nil
}

func execUCI(args ...string) (string, error) {
	out, err := exec.Command("uci", args...).CombinedOutput()
	return string(out), err
}
