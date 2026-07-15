package runtime

import (
	"context"
	"fmt"
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
	mu            sync.Mutex
	plan          *plan.Plan
	control       *control.Control
	registry      *outbound.Registry
	router        *router.Router
	tproxy        *tproxy.Server
	dnsServer     *dns.Server
	listDownload  *listdownload.Server
}

func New(p *plan.Plan, ctrl *control.Control) (*Runtime, error) {
	reg, err := outbound.NewRegistry(p.Outbounds)
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
	if p.ListDownload.Enabled && p.ListDownload.Section != "" {
		rt.listDownload = listdownload.New(p.ListDownload.Section, p.ListDownload.Port, reg, ctrl)
	}
	ctrl.SetSelector(reg)
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
	return nil
}

func (r *Runtime) Stop() {
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
