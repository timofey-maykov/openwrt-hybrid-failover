package outbound

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fragmentRange struct{ min, max int }
type fragmentOptions struct{ lengths, delays []fragmentRange }

// Only the TLS ClientHello fragment mask is implemented. Reject other masks
// explicitly instead of silently sending an unmasked connection.
func parseFinalMask(value any) (*fragmentOptions, error) {
	text, ok := value.(string)
	if !ok || len(text) > 16384 {
		return nil, fmt.Errorf("fm: invalid finalmask")
	}
	var doc struct {
		TCP []struct {
			Type     string `json:"type"`
			Settings struct {
				Packets  string   `json:"packets"`
				Length   string   `json:"length"`
				Interval string   `json:"interval"`
				Delay    string   `json:"delay"`
				Lengths  []string `json:"lengths"`
				Delays   []string `json:"delays"`
			} `json:"settings"`
		} `json:"tcp"`
		UDP []json.RawMessage `json:"udp"`
	}
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err := d.Decode(&doc); err != nil {
		return nil, fmt.Errorf("fm: invalid or unsupported finalmask settings")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("fm: trailing finalmask data")
	}
	if len(doc.TCP) != 1 || len(doc.UDP) != 0 || doc.TCP[0].Type != "fragment" || doc.TCP[0].Settings.Packets != "tlshello" {
		return nil, fmt.Errorf("fm: only one TCP fragment mask with packets=tlshello is supported")
	}
	s := doc.TCP[0].Settings
	lengths := s.Lengths
	if len(lengths) == 0 {
		lengths = []string{s.Length}
	}
	delays := s.Delays
	if len(delays) == 0 {
		if s.Delay != "" && s.Interval != "" && s.Delay != s.Interval {
			return nil, fmt.Errorf("fm: conflicting delay and interval")
		}
		delay := s.Delay
		if delay == "" {
			delay = s.Interval
		}
		if delay == "" {
			delay = "0"
		}
		delays = []string{delay}
	}
	if len(lengths) > 32 || len(delays) > 32 {
		return nil, fmt.Errorf("fm: too many fragment ranges")
	}
	opt := &fragmentOptions{}
	for _, v := range lengths {
		r, err := parseFragmentRange(v, 1, 16384)
		if err != nil {
			return nil, err
		}
		opt.lengths = append(opt.lengths, r)
	}
	for _, v := range delays {
		r, err := parseFragmentRange(v, 0, 1000)
		if err != nil {
			return nil, err
		}
		opt.delays = append(opt.delays, r)
	}
	return opt, nil
}

func parseFragmentRange(text string, min, max int) (fragmentRange, error) {
	a, b, has := strings.Cut(text, "-")
	if !has {
		b = a
	}
	lo, e1 := strconv.Atoi(a)
	hi, e2 := strconv.Atoi(b)
	if e1 != nil || e2 != nil || lo < min || hi < lo || hi > max {
		return fragmentRange{}, fmt.Errorf("fm: invalid fragment range")
	}
	return fragmentRange{lo, hi}, nil
}

func pickFragmentRange(ranges []fragmentRange, index int) (int, error) {
	if index >= len(ranges) {
		index = len(ranges) - 1
	}
	r := ranges[index]
	if r.min == r.max {
		return r.min, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(r.max-r.min+1)))
	if err != nil {
		return 0, err
	}
	return r.min + int(n.Int64()), nil
}

type fragmentConn struct {
	net.Conn
	opt   *fragmentOptions
	mu    sync.Mutex
	wrote bool
	done  chan struct{}
	once  sync.Once
}

func wrapFragment(conn net.Conn, opt *fragmentOptions) net.Conn {
	if opt == nil {
		return conn
	}
	return &fragmentConn{Conn: conn, opt: opt, done: make(chan struct{})}
}
func (c *fragmentConn) Close() error { c.once.Do(func() { close(c.done) }); return c.Conn.Close() }

func writeFragment(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (c *fragmentConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.wrote || len(p) == 0 {
		return c.Conn.Write(p)
	}
	c.wrote = true
	if len(p) < 6 || p[0] != 22 || p[5] != 1 {
		return 0, fmt.Errorf("fm: expected TLS ClientHello")
	}
	recordLen := 5 + (int(p[3])<<8 | int(p[4]))
	if recordLen > len(p) {
		return 0, fmt.Errorf("fm: incomplete TLS ClientHello")
	}
	payload := p[5:recordLen]
	for index := 0; len(payload) > 0; index++ {
		n, err := pickFragmentRange(c.opt.lengths, index)
		if err != nil {
			return 0, err
		}
		if n > len(payload) {
			n = len(payload)
		}
		var record bytes.Buffer
		record.Write(p[:3])
		record.WriteByte(byte(n >> 8))
		record.WriteByte(byte(n))
		record.Write(payload[:n])
		if err := writeFragment(c.Conn, record.Bytes()); err != nil {
			return 0, err
		}
		payload = payload[n:]
		delay, err := pickFragmentRange(c.opt.delays, index)
		if err != nil {
			return 0, err
		}
		if delay > 0 {
			timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
			select {
			case <-timer.C:
			case <-c.done:
				timer.Stop()
				return 0, net.ErrClosed
			}
		}
	}
	if err := writeFragment(c.Conn, p[recordLen:]); err != nil {
		return recordLen, err
	}
	return len(p), nil
}
