package outbound

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

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

// balanceHandler spreads connections over the live members. The member for a
// connection is picked by rendezvous hashing of the site (registrable domain or
// IP), so one site stays on one channel while it lives and moves only when its
// channel goes down; sites that need a stable exit IP keep working.
type balanceHandler struct {
	tag      string
	members  []string
	registry *Registry
}

func (b *balanceHandler) Tag() string  { return b.tag }
func (b *balanceHandler) Close() error { return nil }

func (b *balanceHandler) order(address string) []string {
	ctrl := b.registry.control()
	var live, down []string
	for _, m := range b.members {
		if memberDown(ctrl, m) {
			down = append(down, m)
		} else {
			live = append(live, m)
		}
	}
	key := siteKey(address)
	rank := func(list []string) {
		sort.SliceStable(list, func(i, j int) bool {
			return rendezvous(key, list[i]) > rendezvous(key, list[j])
		})
	}
	rank(live)
	rank(down)
	return append(live, down...)
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
	if len(labels) > 2 {
		labels = labels[len(labels)-2:]
	}
	return strings.Join(labels, ".")
}

func rendezvous(key, member string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(member))
	return h.Sum64()
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
