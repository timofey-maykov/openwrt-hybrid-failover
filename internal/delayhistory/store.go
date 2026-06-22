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

const flushMinInterval = 20 * time.Second

type Sample struct {
	Time    time.Time `json:"time"`
	DelayMs int       `json:"delay_ms"`
	OK      bool      `json:"ok"`
}

type ChannelHistory struct {
	Channel string   `json:"channel"`
	Samples []Sample `json:"samples"`
}

// SampleInput is one delay sample for RecordBatch.
type SampleInput struct {
	DelayMs int
	OK      bool
}

type Store struct {
	path      string
	maxPts    int
	mu        sync.Mutex
	data      []ChannelHistory
	loaded    bool
	dirty     bool
	lastFlush time.Time
}

var defaultStore = NewStore(paths.DelayHistoryFile)

func NewStore(path string) *Store {
	return &Store{path: path, maxPts: maxPoints()}
}

func DefaultStore() *Store {
	return defaultStore
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
	if err := s.ensureLoadedLocked(); err != nil {
		return err
	}
	s.appendSampleLocked(channel, delayMs, ok)
	return s.maybeFlushLocked(false)
}

func (s *Store) RecordBatch(samples map[string]SampleInput) error {
	if len(samples) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return err
	}
	for channel, sample := range samples {
		if channel == "" {
			continue
		}
		s.appendSampleLocked(channel, sample.DelayMs, sample.OK)
	}
	return s.maybeFlushLocked(true)
}

func (s *Store) ReadAll() ([]ChannelHistory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	out := make([]ChannelHistory, len(s.data))
	copy(out, s.data)
	return out, nil
}

// Prune drops history for channels not in keep set.
func (s *Store) Prune(keep map[string]struct{}) error {
	if len(keep) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoadedLocked(); err != nil {
		return err
	}
	filtered := s.data[:0]
	for _, row := range s.data {
		if _, ok := keep[row.Channel]; ok {
			filtered = append(filtered, row)
		}
	}
	if len(filtered) == len(s.data) {
		return nil
	}
	s.data = filtered
	s.dirty = true
	return s.flushLocked()
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *Store) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	if s.maxPts <= 0 {
		s.maxPts = maxPoints()
	}
	data, err := s.readDiskLocked()
	if err != nil {
		return err
	}
	s.data = data
	s.loaded = true
	return nil
}

func (s *Store) appendSampleLocked(channel string, delayMs int, ok bool) {
	now := time.Now().UTC()
	for i := range s.data {
		if s.data[i].Channel != channel {
			continue
		}
		s.data[i].Samples = append(s.data[i].Samples, Sample{
			Time:    now,
			DelayMs: delayMs,
			OK:      ok,
		})
		if len(s.data[i].Samples) > s.maxPts {
			s.data[i].Samples = s.data[i].Samples[len(s.data[i].Samples)-s.maxPts:]
		}
		s.dirty = true
		return
	}
	s.data = append(s.data, ChannelHistory{
		Channel: channel,
		Samples: []Sample{{Time: now, DelayMs: delayMs, OK: ok}},
	})
	s.dirty = true
}

func (s *Store) maybeFlushLocked(force bool) error {
	if !s.dirty {
		return nil
	}
	if !force && !s.lastFlush.IsZero() && time.Since(s.lastFlush) < flushMinInterval {
		return nil
	}
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	if !s.dirty && s.loaded {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	s.dirty = false
	s.lastFlush = time.Now()
	return nil
}

func (s *Store) readDiskLocked() ([]ChannelHistory, error) {
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
	return DefaultStore().ReadAll()
}

func Record(channel string, delayMs int, ok bool) error {
	return DefaultStore().Record(channel, delayMs, ok)
}

func RecordBatch(samples map[string]SampleInput) error {
	return DefaultStore().RecordBatch(samples)
}

func Prune(keep map[string]struct{}) error {
	return DefaultStore().Prune(keep)
}

func Flush() error {
	return DefaultStore().Flush()
}
