package outbound

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/delayhistory"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

type Handler interface {
	Tag() string
	DialTCP(ctx context.Context, network, address string) (net.Conn, error)
	DialUDP(ctx context.Context, network, address string) (net.PacketConn, error)
	Close() error
}

type Registry struct {
	mu        sync.RWMutex
	handlers  map[string]Handler
	selectors map[string]*selectorHandler
	urltests  []*urlTestRunner
}

func NewRegistry(plans []plan.OutboundPlan) (*Registry, error) {
	r := &Registry{
		handlers:  make(map[string]Handler),
		selectors: make(map[string]*selectorHandler),
	}
	for _, p := range plans {
		h, err := newHandler(p)
		if err != nil {
			return nil, fmt.Errorf("outbound %q: %w", p.Tag, err)
		}
		switch p.Kind {
		case plan.OutboundSelector:
			sh := &selectorHandler{
				tag:     p.Tag,
				members: p.Members,
				active:  p.Default,
				registry: r,
			}
			if sh.active == "" && len(p.Members) > 0 {
				sh.active = p.Members[0]
			}
			r.handlers[p.Tag] = sh
			r.selectors[p.Tag] = sh
		case plan.OutboundURLTest:
			ut := newURLTestRunner(p, r)
			r.handlers[p.Tag] = ut
			r.urltests = append(r.urltests, ut)
		default:
			r.handlers[p.Tag] = h
		}
	}
	return r, nil
}

func newHandler(p plan.OutboundPlan) (Handler, error) {
	switch p.Kind {
	case plan.OutboundDirect:
		return &directHandler{tag: p.Tag}, nil
	case plan.OutboundDirectBind, plan.OutboundAWG2Bind:
		return &directHandler{tag: p.Tag, bindIface: p.BindIface}, nil
	case plan.OutboundHysteria2:
		return newHysteria2Handler(p)
	case plan.OutboundVLESS:
		return newVLESSHandler(p)
	case plan.OutboundTrojan, plan.OutboundShadowsocks, plan.OutboundSocks:
		return newProxyHandler(p)
	case plan.OutboundURLTest, plan.OutboundSelector:
		return &directHandler{tag: p.Tag}, nil
	default:
		return nil, fmt.Errorf("unsupported kind %q", p.Kind)
	}
}

func (r *Registry) Handler(tag string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[tag]
	if !ok {
		return nil, fmt.Errorf("outbound %q not found", tag)
	}
	return h, nil
}

func (r *Registry) DialTCP(ctx context.Context, tag, network, address string) (net.Conn, error) {
	h, err := r.Handler(tag)
	if err != nil {
		return nil, err
	}
	return h.DialTCP(ctx, network, address)
}

func (r *Registry) DialUDP(ctx context.Context, tag, network, address string) (net.PacketConn, error) {
	h, err := r.Handler(tag)
	if err != nil {
		return nil, err
	}
	return h.DialUDP(ctx, network, address)
}

func (r *Registry) SelectorActive(section string) (string, error) {
	selectorTag := plan.OutboundTag(section)
	r.mu.RLock()
	sh, ok := r.selectors[selectorTag]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("selector for section %q not found", section)
	}
	sh.mu.RLock()
	active := sh.active
	sh.mu.RUnlock()
	if active == "" {
		return "", fmt.Errorf("selector %q has no active member", selectorTag)
	}
	return active, nil
}

func (r *Registry) SetSelector(section, tag string) error {
	selectorTag := plan.OutboundTag(section)
	r.mu.RLock()
	sh, ok := r.selectors[selectorTag]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("selector for section %q not found", section)
	}
	return sh.SetActive(tag)
}

func (r *Registry) StartURLTests(ctx context.Context, ctrl *control.Control) error {
	for _, ut := range r.urltests {
		if err := ut.Start(ctx, ctrl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Stop() {
	for _, ut := range r.urltests {
		ut.Stop()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.handlers {
		_ = h.Close()
	}
	r.handlers = nil
	r.selectors = nil
	r.urltests = nil
}

// URLTestActive returns the active member tag for a urltest outbound group.
func (r *Registry) URLTestActive(tag string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ut := range r.urltests {
		if ut.plan.Tag != tag {
			continue
		}
		member, err := ut.bestMember()
		if err != nil {
			return ""
		}
		return member
	}
	return ""
}

type directHandler struct {
	tag       string
	bindIface string
}

func (d *directHandler) Tag() string { return d.tag }

func (d *directHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	return outboundDialer(d.bindIface).DialContext(ctx, network, address)
}

func (d *directHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, fmt.Errorf("udp dial not implemented")
}

func (d *directHandler) Close() error { return nil }

type selectorHandler struct {
	mu       sync.RWMutex
	tag      string
	members  []string
	active   string
	registry *Registry
}

func (s *selectorHandler) Tag() string { return s.tag }

func (s *selectorHandler) SetActive(tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.members {
		if m == tag {
			s.active = tag
			return nil
		}
	}
	return fmt.Errorf("tag %q not in selector %q", tag, s.tag)
}

func (s *selectorHandler) activeHandler() (Handler, error) {
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	if active == "" {
		return nil, fmt.Errorf("selector %q has no active member", s.tag)
	}
	return s.registry.Handler(active)
}

func (s *selectorHandler) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	h, err := s.activeHandler()
	if err != nil {
		return nil, err
	}
	return h.DialTCP(ctx, network, address)
}

func (s *selectorHandler) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	h, err := s.activeHandler()
	if err != nil {
		return nil, err
	}
	return h.DialUDP(ctx, network, address)
}

func (s *selectorHandler) Close() error { return nil }

type urlTestRunner struct {
	plan     plan.OutboundPlan
	registry *Registry
	cancel   context.CancelFunc
	mu       sync.RWMutex
	active   string
}

func newURLTestRunner(p plan.OutboundPlan, r *Registry) *urlTestRunner {
	u := &urlTestRunner{plan: p, registry: r}
	if len(p.Members) > 0 {
		u.active = p.Members[0]
	}
	return u
}

func (u *urlTestRunner) Tag() string { return u.plan.Tag }

func (u *urlTestRunner) DialTCP(ctx context.Context, network, address string) (net.Conn, error) {
	best, err := u.bestMember()
	if err != nil {
		return nil, err
	}
	return u.registry.DialTCP(ctx, best, network, address)
}

func (u *urlTestRunner) DialUDP(ctx context.Context, network, address string) (net.PacketConn, error) {
	best, err := u.bestMember()
	if err != nil {
		return nil, err
	}
	h, err := u.registry.Handler(best)
	if err != nil {
		return nil, err
	}
	return h.DialUDP(ctx, network, address)
}

func (u *urlTestRunner) Close() error { return nil }

func (u *urlTestRunner) bestMember() (string, error) {
	if len(u.plan.Members) == 0 {
		return "", fmt.Errorf("urltest %q has no members", u.plan.Tag)
	}
	u.mu.RLock()
	active := u.active
	u.mu.RUnlock()
	if active != "" {
		return active, nil
	}
	return u.plan.Members[0], nil
}

func (u *urlTestRunner) Start(ctx context.Context, ctrl *control.Control) error {
	if u.plan.URLTest == nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	u.cancel = cancel
	go u.loop(runCtx, ctrl)
	return nil
}

func (u *urlTestRunner) Stop() {
	if u.cancel != nil {
		u.cancel()
	}
}

func (u *urlTestRunner) loop(ctx context.Context, ctrl *control.Control) {
	interval := parseDuration(u.plan.URLTest.Interval, 30*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		u.probe(ctrl)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (u *urlTestRunner) probe(ctrl *control.Control) {
	if u.plan.URLTest == nil {
		return
	}
	testURL := u.plan.URLTest.URL
	delays := make(map[string]int, len(u.plan.Members))
	batch := make(map[string]delayhistory.SampleInput, len(u.plan.Members))
	for _, member := range u.plan.Members {
		start := time.Now()
		h, err := u.registry.Handler(member)
		if err != nil {
			ctrl.SetDelay(member, plan.DelaySample{Tag: member, OK: false})
			delays[member] = -1
			batch[member] = delayhistory.SampleInput{OK: false}
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, err := h.DialTCP(ctx, "tcp", extractHostPort(testURL))
		cancel()
		if err != nil {
			ctrl.SetDelay(member, plan.DelaySample{Tag: member, OK: false})
			delays[member] = -1
			batch[member] = delayhistory.SampleInput{OK: false}
			continue
		}
		_ = conn.Close()
		ms := int(time.Since(start).Milliseconds())
		if ms <= 0 {
			ms = 1
		}
		delays[member] = ms
		ctrl.SetDelay(member, plan.DelaySample{
			Tag:   member,
			Delay: time.Duration(ms) * time.Millisecond,
			OK:    true,
		})
		batch[member] = delayhistory.SampleInput{DelayMs: ms, OK: true}
	}
	_ = delayhistory.RecordBatch(batch)
	u.mu.Lock()
	u.active = pickURLTestMember(u.plan, delays, u.active)
	u.mu.Unlock()
}

func pickURLTestMember(p plan.OutboundPlan, delays map[string]int, current string) string {
	tolerance := 50
	if p.URLTest != nil && p.URLTest.Tolerance > 0 {
		tolerance = p.URLTest.Tolerance
	}
	bestTag := ""
	bestMs := -1
	for _, m := range p.Members {
		ms, ok := delays[m]
		if !ok || ms <= 0 {
			continue
		}
		if bestMs < 0 || ms < bestMs {
			bestMs = ms
			bestTag = m
		}
	}
	if bestTag == "" {
		if current != "" {
			return current
		}
		if len(p.Members) > 0 {
			return p.Members[0]
		}
		return ""
	}
	if current != "" {
		if ms, ok := delays[current]; ok && ms > 0 && ms <= bestMs+tolerance {
			return current
		}
	}
	return bestTag
}

func parseDuration(raw string, def time.Duration) time.Duration {
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}

func extractHostPort(rawURL string) string {
	if rawURL == "" {
		return "www.gstatic.com:443"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		host := stringsTrimPrefix(rawURL, "https://")
		host = stringsTrimPrefix(host, "http://")
		if i := indexString(host, "/"); i >= 0 {
			host = host[:i]
		}
		if !stringsContains(host, ":") {
			return host + ":443"
		}
		return host
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
