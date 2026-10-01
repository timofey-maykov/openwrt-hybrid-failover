// Package chanmetrics keeps short time series of every channel (tunnel): rx/tx
// rate, open connections and probe delay. The engine samples every Step and
// writes two files to tmpfs: a fine ring (FineSpan at Step) and a coarse ring
// (CoarseSpan at one minute). LuCI reads the files directly to draw the live
// charts, so polling them costs no process spawn on the router.
package chanmetrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	Step        = 2 * time.Second
	FineSpan    = 10 * time.Minute
	CoarseStep  = time.Minute
	CoarseSpan  = 24 * time.Hour
	fineSize    = int(FineSpan / Step)
	coarseSize  = int(CoarseSpan / CoarseStep)
	FormatV1    = 1
	delayErr    = -1 // probe failed
	delayNoData = 0  // never probed
)

var (
	FineFile   = "/var/run/hybrid-failover/channel-metrics.json"
	CoarseFile = "/var/run/hybrid-failover/channel-metrics-24h.json"
)

// Channel describes one series.
type Channel struct {
	Key     string `json:"key"` // section/id
	Section string `json:"section"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Iface   string `json:"iface,omitempty"`
	Kind    string `json:"kind,omitempty"`
	// Lists bound to this channel by list_route, for the legend.
	Lists []string `json:"lists,omitempty"`
}

// Reading is one raw sample of a channel: monotonic counters plus gauges.
type Reading struct {
	Channel
	RxBytes uint64
	TxBytes uint64
	Conns   uint64 // total opened
	Active  int64
	DelayMs int // >0 ok, -1 failed, 0 unknown
	Up      bool
}

// Series holds the points of one channel, aligned with File.T.
type Series struct {
	Rx     []int64 `json:"rx"`     // bytes/s
	Tx     []int64 `json:"tx"`     // bytes/s
	Active []int64 `json:"active"` // open connections
	New    []int64 `json:"new"`    // connections opened per second ×100
	Delay  []int64 `json:"delay"`  // ms, -1 failed, 0 unknown
}

// Totals are the counters since the engine process started.
type Totals struct {
	Rx     uint64 `json:"rx"`
	Tx     uint64 `json:"tx"`
	Conns  uint64 `json:"conns"`
	Active int64  `json:"active"`
	Delay  int    `json:"delay"`
	Up     bool   `json:"up"`
}

// File is what the engine writes and LuCI reads.
type File struct {
	Format   int               `json:"format"`
	Step     int               `json:"step"` // seconds
	Updated  int64             `json:"updated"`
	Channels []Channel         `json:"channels"`
	T        []int64           `json:"t"` // unix seconds
	Series   map[string]Series `json:"series"`
	Totals   map[string]Totals `json:"totals,omitempty"`
}

type ring struct {
	size   int
	t      []int64
	series map[string]*Series
}

func newRing(size int) *ring { return &ring{size: size, series: make(map[string]*Series)} }

func (r *ring) push(ts int64, vals map[string][5]int64) {
	n := len(r.t)
	r.t = append(r.t, ts)
	for key, v := range vals {
		s, ok := r.series[key]
		if !ok {
			// A channel that appears later is padded so arrays stay aligned.
			s = &Series{
				Rx: make([]int64, n), Tx: make([]int64, n), Active: make([]int64, n),
				New: make([]int64, n), Delay: make([]int64, n),
			}
			r.series[key] = s
		}
		s.Rx = append(s.Rx, v[0])
		s.Tx = append(s.Tx, v[1])
		s.Active = append(s.Active, v[2])
		s.New = append(s.New, v[3])
		s.Delay = append(s.Delay, v[4])
	}
	for key, s := range r.series {
		if _, ok := vals[key]; ok {
			continue
		}
		s.Rx = append(s.Rx, 0)
		s.Tx = append(s.Tx, 0)
		s.Active = append(s.Active, 0)
		s.New = append(s.New, 0)
		s.Delay = append(s.Delay, delayNoData)
	}
	r.trim()
}

func (r *ring) trim() {
	if over := len(r.t) - r.size; over > 0 {
		r.t = r.t[over:]
		for key, s := range r.series {
			s.Rx, s.Tx, s.Active = s.Rx[over:], s.Tx[over:], s.Active[over:]
			s.New, s.Delay = s.New[over:], s.Delay[over:]
			if allZero(s) {
				delete(r.series, key)
			}
		}
	}
}

func allZero(s *Series) bool {
	for i := range s.Rx {
		if s.Rx[i] != 0 || s.Tx[i] != 0 || s.Active[i] != 0 || s.New[i] != 0 || s.Delay[i] != 0 {
			return false
		}
	}
	return true
}

func (r *ring) file(step int, chans []Channel, totals map[string]Totals) File {
	f := File{
		Format: FormatV1, Step: step, Channels: chans, Totals: totals,
		T: append([]int64(nil), r.t...), Series: make(map[string]Series, len(r.series)),
	}
	for k, s := range r.series {
		f.Series[k] = *s
	}
	if n := len(r.t); n > 0 {
		f.Updated = r.t[n-1]
	}
	return f
}

// Sampler turns readings into rates and keeps both rings.
type Sampler struct {
	mu     sync.Mutex
	fine   *ring
	coarse *ring
	last   map[string]Reading
	lastAt time.Time
	chans  []Channel
	totals map[string]Totals
	// minute accumulator
	acc      map[string]*minuteAcc
	accStart time.Time
}

type minuteAcc struct {
	n                  int64
	rx, tx, newConns   int64
	active             int64
	delaySum, delayCnt int64
	delayErr           bool
}

func NewSampler() *Sampler {
	return &Sampler{
		fine:   newRing(fineSize),
		coarse: newRing(coarseSize),
		last:   make(map[string]Reading),
		acc:    make(map[string]*minuteAcc),
	}
}

// Load restores the rings from files written by an earlier run of the
// process (tmpfs, so they survive a service restart, not a reboot).
func (s *Sampler) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, err := Read(FineFile); err == nil && f.Format == FormatV1 {
		loadRing(s.fine, f)
		s.chans = f.Channels
	}
	if f, err := Read(CoarseFile); err == nil && f.Format == FormatV1 {
		loadRing(s.coarse, f)
	}
}

func loadRing(r *ring, f File) {
	r.t = append([]int64(nil), f.T...)
	for k, v := range f.Series {
		v := v
		if len(v.Rx) != len(f.T) || len(v.Tx) != len(f.T) || len(v.Active) != len(f.T) ||
			len(v.New) != len(f.T) || len(v.Delay) != len(f.T) {
			continue
		}
		r.series[k] = &v
	}
	r.trim()
}

// Add records one round of readings taken at now. It returns whether a
// minute closed (the coarse ring changed).
func (s *Sampler) Add(now time.Time, readings []Reading) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	dt := now.Sub(s.lastAt).Seconds()
	first := s.lastAt.IsZero() || dt <= 0 || dt > 30
	vals := make(map[string][5]int64, len(readings))
	chans := make([]Channel, 0, len(readings))
	totals := make(map[string]Totals, len(readings))
	for _, rd := range readings {
		chans = append(chans, rd.Channel)
		totals[rd.Key] = Totals{Rx: rd.RxBytes, Tx: rd.TxBytes, Conns: rd.Conns, Active: rd.Active, Delay: rd.DelayMs, Up: rd.Up}
		prev, seen := s.last[rd.Key]
		var rx, tx, nc int64
		if seen && !first {
			rx = rate(prev.RxBytes, rd.RxBytes, dt)
			tx = rate(prev.TxBytes, rd.TxBytes, dt)
			nc = int64(float64(delta(prev.Conns, rd.Conns))*100/dt + 0.5)
		}
		vals[rd.Key] = [5]int64{rx, tx, rd.Active, nc, int64(rd.DelayMs)}
	}
	s.last = make(map[string]Reading, len(readings))
	for _, rd := range readings {
		s.last[rd.Key] = rd
	}
	s.lastAt = now
	s.chans = chans
	s.totals = totals
	if first {
		return false
	}
	s.fine.push(now.Unix(), vals)
	return s.accumulate(now, vals)
}

func (s *Sampler) accumulate(now time.Time, vals map[string][5]int64) bool {
	minute := now.Truncate(CoarseStep)
	closed := false
	if !s.accStart.IsZero() && minute.After(s.accStart) {
		out := make(map[string][5]int64, len(s.acc))
		for k, a := range s.acc {
			if a.n == 0 {
				continue
			}
			d := int64(delayNoData)
			if a.delayCnt > 0 {
				d = a.delaySum / a.delayCnt
			} else if a.delayErr {
				d = delayErr
			}
			out[k] = [5]int64{a.rx / a.n, a.tx / a.n, a.active, a.newConns / a.n, d}
		}
		s.coarse.push(s.accStart.Unix(), out)
		s.acc = make(map[string]*minuteAcc)
		closed = true
	}
	if s.accStart.IsZero() || minute.After(s.accStart) {
		s.accStart = minute
	}
	for k, v := range vals {
		a := s.acc[k]
		if a == nil {
			a = &minuteAcc{}
			s.acc[k] = a
		}
		a.n++
		a.rx += v[0]
		a.tx += v[1]
		if v[2] > a.active {
			a.active = v[2] // peak open connections of the minute
		}
		a.newConns += v[3]
		switch {
		case v[4] > 0:
			a.delaySum += v[4]
			a.delayCnt++
		case v[4] == delayErr:
			a.delayErr = true
		}
	}
	return closed
}

func delta(prev, cur uint64) uint64 {
	if cur < prev {
		return cur // counter reset: engine restarted or interface recreated
	}
	return cur - prev
}

func rate(prev, cur uint64, dt float64) int64 {
	return int64(float64(delta(prev, cur))/dt + 0.5)
}

// Fine returns the fine ring as a file.
func (s *Sampler) Fine() File {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fine.file(int(Step/time.Second), s.chans, s.totals)
}

// Coarse returns the coarse ring as a file.
func (s *Sampler) Coarse() File {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.coarse.file(int(CoarseStep/time.Second), s.chans, s.totals)
}

// Write stores f at path atomically.
func Write(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Read loads a metrics file.
func Read(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, err
	}
	return f, nil
}
