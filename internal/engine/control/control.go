package control

import (
	"fmt"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

type Selector interface {
	SetSelector(section, tag string) error
	SelectorActive(section string) (string, error)
}

type Control struct {
	mu              sync.RWMutex
	plan            *plan.Plan
	selector        Selector
	activeOverrides map[string]string
	delays          map[string]plan.DelaySample
}

func New() *Control {
	return &Control{
		activeOverrides: make(map[string]string),
		delays:          make(map[string]plan.DelaySample),
	}
}

func (c *Control) BindPlan(p *plan.Plan) {
	c.mu.Lock()
	c.plan = p
	for _, sec := range p.Sections {
		if sec.SelectorTag != "" {
			if _, ok := c.activeOverrides[sec.Name]; !ok {
				c.activeOverrides[sec.Name] = defaultForSection(p, sec.Name)
			}
		}
	}
	c.mu.Unlock()
}

func defaultForSection(p *plan.Plan, section string) string {
	tag := plan.OutboundTag(section)
	for _, ob := range p.Outbounds {
		if ob.Tag == tag && ob.Kind == plan.OutboundSelector {
			if ob.Default != "" {
				return ob.Default
			}
		}
	}
	return tag
}

func (c *Control) SetSelector(sel Selector) {
	c.mu.Lock()
	c.selector = sel
	c.mu.Unlock()
}

func (c *Control) ActiveTag(section string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	selectorTag := plan.OutboundTag(section)
	if c.selector != nil {
		if tag, err := c.selector.SelectorActive(section); err == nil && tag != "" && tag != selectorTag {
			return tag
		}
	}
	if v, ok := c.activeOverrides[section]; ok && v != "" && v != selectorTag {
		return v
	}
	if c.plan != nil {
		if d := defaultForSection(c.plan, section); d != "" && d != selectorTag {
			return d
		}
	}
	return selectorTag
}

func (c *Control) ActiveBySection() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]string, len(c.activeOverrides))
	for k, v := range c.activeOverrides {
		out[k] = v
	}
	return out
}

func (c *Control) SwitchOutbound(section, tag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.plan == nil {
		return fmt.Errorf("no plan")
	}
	if !outboundExists(c.plan, tag) {
		return fmt.Errorf("outbound %q not found", tag)
	}
	c.activeOverrides[section] = tag
	if c.selector != nil {
		return c.selector.SetSelector(section, tag)
	}
	return nil
}

func (c *Control) Delay(tag string) plan.DelaySample {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if d, ok := c.delays[tag]; ok {
		return d
	}
	return plan.DelaySample{Tag: tag}
}

func (c *Control) SetDelay(tag string, d plan.DelaySample) {
	c.mu.Lock()
	c.delays[tag] = d
	c.mu.Unlock()
}

func (c *Control) AllDelays() map[string]plan.DelaySample {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]plan.DelaySample, len(c.delays))
	for k, v := range c.delays {
		out[k] = v
	}
	return out
}

func outboundExists(p *plan.Plan, tag string) bool {
	for _, ob := range p.Outbounds {
		if ob.Tag == tag {
			return true
		}
	}
	return false
}
