package outbound

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

// memberDown reports whether the last urltest probe of tag failed. A member
// with no probe yet (or one that is never probed, like a selector) counts as
// alive: binding to a channel must not black-hole traffic before the first
// probe round.
func memberDown(ctrl *control.Control, tag string) bool {
	if ctrl == nil {
		return false
	}
	d := ctrl.Delay(tag)
	return !d.OK && d.Error != ""
}

// fallbackHandler dials the first live member in order: the bound channel,
// then what the binding falls back to (the section pool or direct). A dial
// error on one member moves on to the next.
type fallbackHandler struct {
	tag      string
	members  []string
	registry *Registry
}

func (f *fallbackHandler) Tag() string  { return f.tag }
func (f *fallbackHandler) Close() error { return nil }

func (f *fallbackHandler) order() []string {
	ctrl := f.registry.control()
	var live, down []string
	for _, m := range f.members {
		if memberDown(ctrl, m) {
			down = append(down, m)
		} else {
			live = append(live, m)
		}
	}
	if len(live) == 0 {
		// Nothing is known to work. With on_down=block the list has only
		// the channel: try it anyway rather than fail without a dial.
		return down
	}
	return live
}

// Active returns the member a new connection would use now.
func (f *fallbackHandler) Active() string {
	if o := f.order(); len(o) > 0 {
		return o[0]
	}
	return ""
}

func (f *fallbackHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	return dialMembers(f.tag, f.order(), func(tag string) (net.Conn, error) {
		return f.registry.DialTCP(ctx, tag, network, address)
	})
}

func (f *fallbackHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return dialMembers(f.tag, f.order(), func(tag string) (net.PacketConn, error) {
		return f.registry.DialUDP(ctx, tag, network, address)
	})
}

// balanceHandler spreads sites over the live members. The member for a
// connection is picked by weighted rendezvous hashing of the site (see
// siteKey), so one site stays on one channel while it lives and moves only
// when its channel goes down or changes speed tier; sites that need a stable
// exit IP keep working. Faster channels get a larger share of sites, and a
// channel far slower than the best one gets no new sites while others are up.
type balanceHandler struct {
	tag      string
	members  []string
	registry *Registry

	mu    sync.Mutex
	tiers map[string]int // last tier per member, for hysteresis
}

func (b *balanceHandler) Tag() string  { return b.tag }
func (b *balanceHandler) Close() error { return nil }

// Speed tiers of a balance member relative to the fastest live member.
const (
	tierFast   = 0
	tierMedium = 1
	tierSlow   = 2
)

// A member is fast up to tierFastRatio x the best delay (or within
// tierFastSlack of it, small absolute gaps do not matter), medium up to
// tierMediumRatio x, slow above. tierHysteresis keeps a member in its tier
// until the ratio crosses the bound by that factor, so probe jitter near a
// bound does not move sites back and forth.
const (
	tierFastRatio   = 1.5
	tierMediumRatio = 2.5
	tierFastSlack   = 100 * time.Millisecond
	tierHysteresis  = 1.15
)

var tierWeight = [...]float64{tierFast: 1, tierMedium: 0.5}

func classifyTier(delay, best time.Duration, prev int, hasPrev bool) int {
	if delay <= 0 || best <= 0 {
		return tierFast // not probed yet: do not starve it
	}
	ratio := float64(delay) / float64(best)
	fastBound, medBound := tierFastRatio, tierMediumRatio
	if hasPrev {
		switch prev {
		case tierFast:
			fastBound *= tierHysteresis
			medBound *= tierHysteresis
		case tierMedium:
			fastBound /= tierHysteresis
			medBound *= tierHysteresis
		case tierSlow:
			fastBound /= tierHysteresis
			medBound /= tierHysteresis
		}
	}
	slack := tierFastSlack
	if hasPrev && prev != tierFast {
		slack = time.Duration(float64(slack) / tierHysteresis)
	}
	switch {
	case ratio <= fastBound || delay-best <= slack:
		return tierFast
	case ratio <= medBound:
		return tierMedium
	default:
		return tierSlow
	}
}

type balanceMember struct {
	tag   string
	down  bool
	delay time.Duration
}

func (b *balanceHandler) snapshot() []balanceMember {
	ctrl := b.registry.control()
	out := make([]balanceMember, 0, len(b.members))
	for _, m := range b.members {
		bm := balanceMember{tag: m, down: memberDown(ctrl, m)}
		if ctrl != nil {
			if d := ctrl.Delay(m); d.OK {
				bm.delay = d.Delay
			}
		}
		out = append(out, bm)
	}
	return out
}

func (b *balanceHandler) order(address string) []string {
	return b.rank(siteKey(address), b.snapshot())
}

// rank orders members for one site: fast and medium live members by weighted
// rendezvous score, then slow live members, then members whose last probe
// failed. Dial errors fall through the list in this order.
func (b *balanceHandler) rank(key string, members []balanceMember) []string {
	var best time.Duration
	for _, m := range members {
		if !m.down && m.delay > 0 && (best == 0 || m.delay < best) {
			best = m.delay
		}
	}
	b.mu.Lock()
	if b.tiers == nil {
		b.tiers = make(map[string]int)
	}
	type scored struct {
		tag   string
		score float64
	}
	var picked, slow, down []scored
	for _, m := range members {
		if m.down {
			down = append(down, scored{m.tag, float64(rendezvous(key, m.tag))})
			continue
		}
		prev, ok := b.tiers[m.tag]
		t := classifyTier(m.delay, best, prev, ok)
		if m.delay > 0 {
			b.tiers[m.tag] = t
		}
		if t == tierSlow {
			slow = append(slow, scored{m.tag, float64(rendezvous(key, m.tag))})
			continue
		}
		picked = append(picked, scored{m.tag, weightedScore(key, m.tag, tierWeight[t])})
	}
	b.mu.Unlock()

	out := make([]string, 0, len(members))
	for _, group := range [][]scored{picked, slow, down} {
		sort.SliceStable(group, func(i, j int) bool { return group[i].score > group[j].score })
		for _, s := range group {
			out = append(out, s.tag)
		}
	}
	return out
}

// weightedScore is weighted rendezvous hashing: a member with weight w wins
// a share of keys proportional to w.
func weightedScore(key, member string, w float64) float64 {
	u := (float64(rendezvous(key, member)>>11) + 0.5) / (1 << 53)
	return -w / math.Log(u)
}

func (b *balanceHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	return dialMembers(b.tag, b.order(address), func(tag string) (net.Conn, error) {
		return b.registry.DialTCP(ctx, tag, network, address)
	})
}

func (b *balanceHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return dialMembers(b.tag, b.order(address), func(tag string) (net.PacketConn, error) {
		return b.registry.DialUDP(ctx, tag, network, address)
	})
}

func dialMembers[T any](group string, order []string, dial func(tag string) (T, error)) (T, error) {
	var zero T
	if len(order) == 0 {
		return zero, fmt.Errorf("%s: no members", group)
	}
	var errs []error
	for _, tag := range order {
		c, err := dial(tag)
		if err == nil {
			return c, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", tag, err))
	}
	return zero, fmt.Errorf("%s: %w", group, errors.Join(errs...))
}

// siteKey: last two labels of a host name (three for short second-level ones
// like co.uk or com.ru are not worth a suffix list here; a site split over two
// channels still works), or the IP itself.
// perHostSites are CDNs where one registrable domain carries the traffic of
// a whole service through many servers. Keying them by the full host spreads
// that service over the channels, while one server (one video, one media
// shard) still stays on one channel.
var perHostSites = map[string]bool{
	"googlevideo.com":  true, // YouTube video
	"cdninstagram.com": true,
	"fbcdn.net":        true,
	"telesco.pe":       true, // Telegram media
	"ttvnw.net":        true, // Twitch video
	"nflxvideo.net":    true, // Netflix video
}

// Second-level labels used under country TLDs (example.co.uk, example.com.au).
var secondLevelLabels = map[string]bool{
	"co": true, "com": true, "net": true, "org": true, "gov": true, "edu": true, "ac": true,
}

// siteKey is what one channel is kept for: the registrable domain of the
// host (or the IP), the full host for perHostSites.
func siteKey(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if net.ParseIP(host) != nil {
		return host
	}
	labels := strings.Split(host, ".")
	n := 2
	if len(labels) >= 3 && len(labels[len(labels)-1]) == 2 && secondLevelLabels[labels[len(labels)-2]] {
		n = 3
	}
	if len(labels) <= n {
		return host
	}
	site := strings.Join(labels[len(labels)-n:], ".")
	if perHostSites[site] {
		return host
	}
	return site
}

func rendezvous(key, member string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(member))
	return mix64(h.Sum64())
}

// mix64 is the splitmix64 finalizer. FNV alone leaves the high bits of
// hashes for similar member names correlated, which skews the weighted pick.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

// TrafficStat is what went through one leaf outbound since the engine started.
// Rx/Tx stay zero for interface-bound outbounds (AWG, VPN): their connections
// are spliced in the kernel, the interface counters carry the bytes.
type TrafficStat struct {
	Conns  uint64
	Active int64
	Rx     uint64
	Tx     uint64
}

type trafficCounter struct {
	conns, rx, tx atomic.Uint64
	active        atomic.Int64
}

func (t *trafficCounter) stat() TrafficStat {
	return TrafficStat{Conns: t.conns.Load(), Active: t.active.Load(), Rx: t.rx.Load(), Tx: t.tx.Load()}
}

// countingHandler wraps a leaf outbound to count its connections.
type countingHandler struct {
	Handler
	ctr        *trafficCounter
	countBytes bool
}

func newCountingHandler(h Handler, kind plan.OutboundKind) *countingHandler {
	return &countingHandler{
		Handler:    h,
		ctr:        &trafficCounter{},
		countBytes: kind != plan.OutboundDirectBind && kind != plan.OutboundAWG2Bind,
	}
}

func (c *countingHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := c.Handler.DialTCP(ctx, network, address)
	if err != nil {
		return nil, err
	}
	c.ctr.conns.Add(1)
	c.ctr.active.Add(1)
	if tc, ok := conn.(*net.TCPConn); ok && !c.countBytes {
		// Keep *net.TCPConn methods (ReadFrom/WriteTo) so io.Copy still splices.
		return &trackedTCPConn{TCPConn: tc, ctr: c.ctr}, nil
	}
	return &countingConn{Conn: conn, ctr: c.ctr}, nil
}

func (c *countingHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	pc, err := c.Handler.DialUDP(ctx, network, address)
	if err != nil {
		return nil, err
	}
	c.ctr.conns.Add(1)
	c.ctr.active.Add(1)
	return &countingPacketConn{PacketConn: pc, ctr: c.ctr}, nil
}

type trackedTCPConn struct {
	*net.TCPConn
	ctr  *trafficCounter
	once sync.Once
}

func (t *trackedTCPConn) Close() error {
	t.once.Do(func() { t.ctr.active.Add(-1) })
	return t.TCPConn.Close()
}

type countingConn struct {
	net.Conn
	ctr  *trafficCounter
	once sync.Once
}

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.ctr.rx.Add(uint64(n))
	return n, err
}

func (c *countingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.ctr.tx.Add(uint64(n))
	return n, err
}

func (c *countingConn) Close() error {
	c.once.Do(func() { c.ctr.active.Add(-1) })
	return c.Conn.Close()
}

type countingPacketConn struct {
	net.PacketConn
	ctr  *trafficCounter
	once sync.Once
}

func (c *countingPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	c.ctr.rx.Add(uint64(n))
	return n, addr, err
}

func (c *countingPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	n, err := c.PacketConn.WriteTo(p, addr)
	c.ctr.tx.Add(uint64(n))
	return n, err
}

func (c *countingPacketConn) Close() error {
	c.once.Do(func() { c.ctr.active.Add(-1) })
	return c.PacketConn.Close()
}
