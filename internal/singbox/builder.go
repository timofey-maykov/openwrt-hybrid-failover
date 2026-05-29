package singbox

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type sectionDomainResolver struct {
	tag            string
	dnsType        string
	dnsServer      string
	domainResolver string
	detour         string
}

type Builder struct {
	cfg                    *Config
	pkg                    *uci.Package
	sectionDomainResolvers []sectionDomainResolver
}

func NewBuilder(pkg *uci.Package) *Builder {
	return &Builder{cfg: NewConfig(), pkg: pkg}
}

func (b *Builder) Build() (*Config, error) {
	b.configureLog()
	b.configureInbounds()
	if err := b.configureOutbounds(); err != nil {
		return nil, err
	}
	b.configureDNS()
	b.configureRoute()
	b.configureExperimental()
	sanitizeInternalTags(b.cfg)
	return b.cfg, nil
}

func (b *Builder) configureLog() {
	b.cfg.Log = map[string]any{
		"disabled":  false,
		"level":     "info",
		"timestamp": false,
	}
}

func (b *Builder) configureInbounds() {
	b.cfg.AddInbound(map[string]any{
		"type":                       "tproxy",
		"tag":                        TPROXYInboundTag,
		"listen":                     "::",
		"listen_port":                1602,
		"sniff":                      true,
		"sniff_override_destination": true,
	})
	b.cfg.AddInbound(map[string]any{
		"type":        "direct",
		"tag":         DNSInboundTag,
		"listen":      DNSInboundAddress,
		"listen_port": DNSInboundPort,
	})
}
