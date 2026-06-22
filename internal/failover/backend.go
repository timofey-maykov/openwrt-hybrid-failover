package failover

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

// Backend switches outbounds and reads delays (Clash API or native engine control).
type Backend interface {
	ActiveOutbound(ctx context.Context, selectorTag string) (string, error)
	SwitchProxy(ctx context.Context, selectorTag, outbound string) error
	ProxyDelay(ctx context.Context, tag, testURL string) (int, error)
}

type clashBackend struct {
	cli *clash.Client
}

func NewClashBackend(baseURL string, timeout time.Duration) Backend {
	return &clashBackend{cli: clash.New(baseURL, timeout)}
}

func (b *clashBackend) ActiveOutbound(ctx context.Context, selectorTag string) (string, error) {
	return b.cli.ActiveOutbound(ctx, selectorTag)
}

func (b *clashBackend) SwitchProxy(ctx context.Context, selectorTag, outbound string) error {
	return b.cli.SwitchProxy(ctx, selectorTag, outbound)
}

func (b *clashBackend) ProxyDelay(ctx context.Context, tag, testURL string) (int, error) {
	return b.cli.ProxyDelay(ctx, tag, testURL)
}

type engineBackend struct {
	ctrl *controlWrapper
}

func NewEngineBackend() Backend {
	return &engineBackend{ctrl: &controlWrapper{engine.Default().Control()}}
}

type controlWrapper struct {
	c *control.Control
}

func (w *controlWrapper) Control() *control.Control {
	if w == nil || w.c == nil {
		return engine.Default().Control()
	}
	return w.c
}

func (b *engineBackend) ActiveOutbound(ctx context.Context, selectorTag string) (string, error) {
	_ = ctx
	section, err := sectionFromSelectorTag(selectorTag)
	if err != nil {
		return "", err
	}
	tag := engine.Default().SelectorActive(section)
	if tag == "" {
		return "", fmt.Errorf("no active outbound for section %q", section)
	}
	return tag, nil
}

func (b *engineBackend) SwitchProxy(ctx context.Context, selectorTag, outbound string) error {
	_ = ctx
	section, err := sectionFromSelectorTag(selectorTag)
	if err != nil {
		return err
	}
	return b.ctrl.Control().SwitchOutbound(section, outbound)
}

func (b *engineBackend) ProxyDelay(ctx context.Context, tag, testURL string) (int, error) {
	_ = ctx
	_ = testURL
	d := b.ctrl.Control().Delay(tag)
	if !d.OK {
		return 0, fmt.Errorf("no delay for %q", tag)
	}
	ms := int(d.Delay.Milliseconds())
	if ms <= 0 {
		return 1, nil
	}
	return ms, nil
}

func sectionFromSelectorTag(selectorTag string) (string, error) {
	selectorTag = strings.TrimSpace(selectorTag)
	if !strings.HasSuffix(selectorTag, "-out") {
		return "", fmt.Errorf("invalid selector tag %q", selectorTag)
	}
	section := strings.TrimSuffix(selectorTag, "-out")
	if section == "" {
		return "", fmt.Errorf("empty section in selector tag %q", selectorTag)
	}
	return section, nil
}

// SelectorTagForSection returns the selector outbound tag for a routing section.
func SelectorTagForSection(section string) string {
	return plan.OutboundTag(section)
}
