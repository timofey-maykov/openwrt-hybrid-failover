package engine

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

const (
	TPROXYPort         = plan.TPROXYPort
	DNSListenAddr      = plan.DNSListenAddr
	DNSListenPort      = plan.DNSListenPort
	FakeIPRange        = plan.FakeIPRange
	FakeIPTestDomain   = plan.FakeIPTestDomain
	CheckProxyIPDomain = plan.CheckProxyIPDomain
	DirectTag          = plan.DirectTag
	ModeNative         = plan.ModeNative
	ModeSingbox        = plan.ModeSingbox
)

type (
	Plan             = plan.Plan
	DNSPlan          = plan.DNSPlan
	SectionPlan      = plan.SectionPlan
	OutboundPlan     = plan.OutboundPlan
	OutboundKind     = plan.OutboundKind
	URLTestPlan      = plan.URLTestPlan
	RouteRule        = plan.RouteRule
	RuleSet          = plan.RuleSet
	ListDownloadPlan = plan.ListDownloadPlan
	ConnMeta         = plan.ConnMeta
	DelaySample      = plan.DelaySample
)

const (
	OutboundDirect      = plan.OutboundDirect
	OutboundDirectBind  = plan.OutboundDirectBind
	OutboundVLESS       = plan.OutboundVLESS
	OutboundTrojan      = plan.OutboundTrojan
	OutboundShadowsocks = plan.OutboundShadowsocks
	OutboundSocks       = plan.OutboundSocks
	OutboundHysteria2   = plan.OutboundHysteria2
	OutboundAWG2Bind    = plan.OutboundAWG2Bind
	OutboundURLTest     = plan.OutboundURLTest
	OutboundSelector    = plan.OutboundSelector
)

var (
	CompilePlan  = plan.CompilePlan
	ValidatePlan = plan.ValidatePlan
	EngineMode   = plan.EngineMode
	NativeEnabled = plan.NativeEnabled
	LoadEngineMode = plan.LoadEngineMode
	OutboundTag  = plan.OutboundTag
	URLTestTag   = plan.URLTestTag
	AWGTag       = plan.AWGTag
	PeerTag      = plan.PeerTag
	RulesetTag   = plan.RulesetTag
)
