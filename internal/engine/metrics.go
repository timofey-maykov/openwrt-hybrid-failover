package engine

import (
	"log"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/chanmetrics"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

// sampleChannels runs for the life of the process: every chanmetrics.Step it
// reads the channel counters of the running engine and writes the series LuCI
// draws. Between engine restarts it keeps the history in memory.
func (e *Engine) sampleChannels() {
	s := chanmetrics.NewSampler()
	s.Load()
	t := time.NewTicker(chanmetrics.Step)
	defer t.Stop()
	for now := range t.C {
		readings := e.channelReadings()
		if readings == nil {
			continue
		}
		minuteClosed := s.Add(now, readings)
		if err := chanmetrics.Write(chanmetrics.FineFile, s.Fine()); err != nil {
			log.Printf("hybrid-failover metrics: %v", err)
		}
		if minuteClosed {
			_ = chanmetrics.Write(chanmetrics.CoarseFile, s.Coarse())
		}
	}
}

func (e *Engine) channelReadings() []chanmetrics.Reading {
	e.mu.RLock()
	rt, ctrl, p := e.rt, e.ctrl, e.plan
	e.mu.RUnlock()
	if rt == nil || p == nil {
		return nil
	}
	chans := channelRuntime(p, rt.Traffic())
	kinds := make(map[string]plan.OutboundKind, len(p.Outbounds))
	for _, ob := range p.Outbounds {
		kinds[ob.Tag] = ob.Kind
	}
	bound := make(map[string][]string)
	for _, b := range p.Bindings {
		if b.ChannelTag != "" && !b.Missing {
			bound[b.ChannelTag] = append(bound[b.ChannelTag], b.Lists...)
		}
	}
	out := make([]chanmetrics.Reading, 0, len(chans))
	for _, ch := range chans {
		d := ctrl.Delay(ch.Tag)
		delay := 0
		switch {
		case d.OK && d.Delay > 0:
			delay = int(d.Delay.Milliseconds())
			if delay == 0 {
				delay = 1
			}
		case d.Error != "":
			delay = -1
		}
		out = append(out, chanmetrics.Reading{
			Channel: chanmetrics.Channel{
				Key: ch.Section + "/" + ch.ID, Section: ch.Section, ID: ch.ID, Name: ch.Name,
				Tag: ch.Tag, Iface: ch.Iface, Kind: channelKind(kinds[ch.Tag]), Lists: bound[ch.Tag],
			},
			RxBytes: ch.Rx, TxBytes: ch.Tx, Conns: ch.Conns, Active: ch.Active,
			DelayMs: delay, Up: delay >= 0,
		})
	}
	return out
}

func channelKind(k plan.OutboundKind) string {
	switch k {
	case plan.OutboundAWG2Bind:
		return "awg2"
	case plan.OutboundDirectBind:
		return "vpn"
	case plan.OutboundSelector, plan.OutboundURLTest:
		return "pool"
	default:
		return strings.ToLower(string(k))
	}
}
