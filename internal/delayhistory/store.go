package delayhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Sample struct {
	Time    time.Time `json:"time"`
	DelayMs int       `json:"delay_ms"`
	OK      bool      `json:"ok"`
}

type ChannelHistory struct {
	Channel string   `json:"channel"`
	Samples []Sample `json:"samples"`
}

type Store struct {
	path   string
	maxPts int
	mu     sync.Mutex
}

func DefaultStore() *Store {
	return &Store{path: paths.DelayHistoryFile, maxPts: maxPoints()}
}

func maxPoints() int {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return 50
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return 50
	}
	n, _ := strconv.Atoi(strings.TrimSpace(settings.Get("delay_history_points", "50")))
	if n < 10 {
		return 10
	}
	if n > 200 {
		return 200
	}
	return n
}

func (s *Store) Record(channel string, delayMs int, ok bool) error {
	if channel == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxPts <= 0 {
		s.maxPts = maxPoints()
	}
	data, _ := s.readAll()
	found := false
	for i := range data {
		if data[i].Channel != channel {
			continue
		}
		data[i].Samples = append(data[i].Samples, Sample{
			Time:    time.Now().UTC(),
			DelayMs: delayMs,
			OK:      ok,
		})
		if len(data[i].Samples) > s.maxPts {
			data[i].Samples = data[i].Samples[len(data[i].Samples)-s.maxPts:]
		}
		found = true
		break
	}
	if !found {
		data = append(data, ChannelHistory{
			Channel: channel,
			Samples: []Sample{{Time: time.Now().UTC(), DelayMs: delayMs, OK: ok}},
		})
	}
	return s.writeAll(data)
}

func (s *Store) readAll() ([]ChannelHistory, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var data []ChannelHistory
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func ReadAll() ([]ChannelHistory, error) {
	return DefaultStore().readAll()
}

func (s *Store) writeAll(data []ChannelHistory) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func Record(channel string, delayMs int, ok bool) error {
	return DefaultStore().Record(channel, delayMs, ok)
}
