package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/delayhistory"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/runtime"
)

var defaultEngine atomic.Pointer[Engine]

type Engine struct {
	mu      sync.RWMutex
	plan    *plan.Plan
	running bool
	ctrl    *control.Control
	runCtx  context.Context
	cancel  context.CancelFunc
	rt      *runtime.Runtime
}

func Default() *Engine {
	if e := defaultEngine.Load(); e != nil {
		return e
	}
	e := &Engine{ctrl: control.New()}
	defaultEngine.Store(e)
	return e
}

func (e *Engine) Control() *control.Control {
	return e.ctrl
}

func (e *Engine) Plan() *plan.Plan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.plan
}

func (e *Engine) ApplyPlan(p *plan.Plan) error {
	if err := plan.ValidatePlan(p); err != nil {
		return err
	}
	e.mu.Lock()
	e.plan = p
	e.ctrl.BindPlan(p)
	e.mu.Unlock()
	keep := make(map[string]struct{}, len(p.Outbounds))
	for _, ob := range p.Outbounds {
		keep[ob.Tag] = struct{}{}
	}
	_ = delayhistory.Prune(keep)
	return nil
}

func (e *Engine) Running() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

func (e *Engine) Run(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		if err := e.waitUntilStopped(3 * time.Second); err != nil {
			return err
		}
		e.mu.Lock()
	}
	if e.plan == nil {
		e.mu.Unlock()
		return fmt.Errorf("engine: no plan applied")
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.runCtx = runCtx
	e.cancel = cancel
	e.running = true
	planCopy := e.plan
	e.mu.Unlock()

	rt, err := runtime.New(planCopy, e.ctrl)
	if err != nil {
		e.mu.Lock()
		e.running = false
		e.cancel = nil
		e.mu.Unlock()
		return err
	}
	e.mu.Lock()
	e.rt = rt
	e.mu.Unlock()
	if err := rt.Start(runCtx); err != nil {
		e.mu.Lock()
		e.running = false
		e.cancel = nil
		e.rt = nil
		e.mu.Unlock()
		return err
	}
	markRunningState(true)

	<-runCtx.Done()
	// DNS Stop force-closes after ~2.5s; wait a bit longer so the next Start can bind.
	stopRuntime(rt, 5*time.Second)
	markRunningState(false)
	e.mu.Lock()
	e.running = false
	e.rt = nil
	e.mu.Unlock()
	return nil
}

func (e *Engine) SelectorActive(section string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rt := e.rt
	ctrl := e.ctrl
	if rt != nil {
		if tag, err := rt.SelectorActive(section); err == nil && tag != "" && tag != plan.OutboundTag(section) {
			return tag
		}
	}
	if ctrl == nil {
		return plan.OutboundTag(section)
	}
	return ctrl.ActiveTag(section)
}

func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	rt := e.rt
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := e.waitUntilStopped(5 * time.Second); err != nil {
		// Run() may have timed out before rt.Stop finished; force cleanup with a bound.
		// Never call rt.Stop synchronously without a timeout (would freeze sync/watchdog).
		e.mu.Lock()
		if e.rt != nil {
			rt = e.rt
		}
		e.rt = nil
		e.running = false
		e.cancel = nil
		e.mu.Unlock()
		markRunningState(false)
		if rt != nil {
			stopRuntime(rt, 3*time.Second)
		}
		return
	}
	markRunningState(false)
}

func stopRuntime(rt *runtime.Runtime, timeout time.Duration) {
	if rt == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.Stop()
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func (e *Engine) waitUntilStopped(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.RLock()
		running := e.running
		e.mu.RUnlock()
		if !running {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("engine: stop timeout")
}

func (e *Engine) Reload(ctx context.Context, p *plan.Plan) error {
	e.Stop()
	if err := e.ApplyPlan(p); err != nil {
		return err
	}
	if ctx != nil {
		return e.Run(ctx)
	}
	return nil
}

func (e *Engine) State() State {
	e.mu.RLock()
	defer e.mu.RUnlock()
	st := State{
		Running: e.running,
		Mode:    plan.ModeNative,
	}
	if e.plan != nil {
		st.SectionCount = len(e.plan.Sections)
		st.OutboundCount = len(e.plan.Outbounds)
	}
	if e.ctrl != nil {
		st.ActiveBySection = e.ctrl.ActiveBySection()
	}
	return st
}

type State struct {
	Running         bool              `json:"running"`
	Mode            string            `json:"mode"`
	SectionCount    int               `json:"section_count"`
	OutboundCount   int               `json:"outbound_count"`
	ActiveBySection map[string]string `json:"active_by_section,omitempty"`
}
