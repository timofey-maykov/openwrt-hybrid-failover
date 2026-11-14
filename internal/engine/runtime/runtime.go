package runtime

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/dns"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/listdownload"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/router"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/tproxy"
)

type Runtime struct {
	mu                   sync.Mutex
	plan                 *plan.Plan
	control              *control.Control
	registry             *outbound.Registry
	router               *router.Router
	tproxy               *tproxy.Server
	dnsServer            *dns.Server
	listDownload         *listdownload.Server
	subscriptionDownload *listdownload.Server
}

func New(ctx context.Context, p *plan.Plan, ctrl *control.Control) (*Runtime, error) {
	reg, err := outbound.NewRegistry(ctx, p.Outbounds)
	if err != nil {
		return nil, err
	}
	rtr, err := router.New(p, reg, ctrl)
	if err != nil {
		return nil, err
	}
	dnsPlan := dns.Plan{
		RewriteTTL:    p.DNS.RewriteTTL,
		Bootstrap:     p.DNS.Bootstrap,
		UpstreamVia:   p.DNS.UpstreamVia,
		RejectHTTPS:   p.DNS.RejectHTTPS,
		FakeIPDomains: p.DNS.FakeIPDomains,
		ListenAddr:    plan.DNSListenAddr,
		ListenPort:    plan.DNSListenPort,
	}
	rt := &Runtime{
		plan:      p,
		control:   ctrl,
		registry:  reg,
		router:    rtr,
		tproxy:    tproxy.NewServer(rtr),
		dnsServer: dns.NewServer(dnsPlan, rtr),
	}
	rt.tproxy.DisableQUIC = p.DisableQUIC
	rt.dnsServer.SetUpstreamDialer(rtr)
	if p.ListDownload.Enabled && p.ListDownload.Section != "" {
		rt.listDownload = listdownload.New(p.ListDownload.Section, p.ListDownload.Port, reg, ctrl)
	}
	ctrl.SetSelector(reg)
	rt.subscriptionDownload = listdownload.NewSubscription(subscriptionChannels(p), reg)
	return rt, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := r.dnsServer.Start(ctx); err != nil {
		return fmt.Errorf("dns: %w", err)
	}
	if err := r.tproxy.Start(ctx); err != nil {
		_ = r.dnsServer.Stop()
		return fmt.Errorf("tproxy: %w", err)
	}
	if r.listDownload != nil {
		if err := r.listDownload.Start(ctx); err != nil {
			_ = r.tproxy.Stop()
			_ = r.dnsServer.Stop()
			return fmt.Errorf("list-download: %w", err)
		}
	}
	if err := r.registry.StartURLTests(ctx, r.control); err != nil {
		_ = r.tproxy.Stop()
		_ = r.dnsServer.Stop()
		return fmt.Errorf("urltest: %w", err)
	}
	if err := r.subscriptionDownload.Start(ctx); err != nil {
		// This optional download helper must not take down LAN routing.
		log.Printf("hybrid-failover: subscription tunnel fallback unavailable: %v", err)
	}
	return nil
}

func (r *Runtime) Stop() {
	if r.subscriptionDownload != nil {
		r.subscriptionDownload.Stop()
	}
	if r.listDownload != nil {
		r.listDownload.Stop()
	}
	if r.tproxy != nil {
		_ = r.tproxy.Stop()
	}
	if r.dnsServer != nil {
		_ = r.dnsServer.Stop()
	}
	if r.registry != nil {
		r.registry.Stop()
	}
}

func (r *Runtime) SelectorActive(section string) (string, error) {
	if r.registry == nil {
		return "", fmt.Errorf("runtime: no registry")
	}
	return r.registry.SelectorActive(section)
}

func (r *Runtime) SetSelector(section, tag string) error {
	return r.registry.SetSelector(section, tag)
}

func (r *Runtime) URLTestActive(section string) string {
	if r.registry == nil {
		return ""
	}
	return r.registry.URLTestActive(plan.URLTestTag(section))
}

// Traffic returns the counters of every leaf outbound.
func (r *Runtime) Traffic() map[string]outbound.TrafficStat {
	if r.registry == nil {
		return nil
	}
	return r.registry.Traffic()
}

// BindingActive returns the outbound a list binding dials now: the member of
// its fallback group, or the group tag itself for balance/direct.
func (r *Runtime) BindingActive(tag string) string {
	if r.registry == nil || tag == "" {
		return ""
	}
	if m := r.registry.FallbackActive(tag); m != "" {
		return m
	}
	return tag
}

// Retry concrete tunnel outbounds, including JSON-configured proxies. Selectors
// and their direct fallback must not turn a tunnel retry into another WAN try.
func subscriptionChannels(p *plan.Plan) []plan.ChannelPlan {
	var channels []plan.ChannelPlan
	for _, ob := range p.Outbounds {
		switch ob.Kind {
		case plan.OutboundVLESS, plan.OutboundTrojan, plan.OutboundShadowsocks, plan.OutboundSocks, plan.OutboundHysteria2:
			channels = append(channels, plan.ChannelPlan{Tag: ob.Tag})
		case plan.OutboundDirectBind, plan.OutboundAWG2Bind:
			if ob.BindIface != "" {
				channels = append(channels, plan.ChannelPlan{Tag: ob.Tag})
			}
		}
	}
	return channels
}
