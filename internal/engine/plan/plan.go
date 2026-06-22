package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	TPROXYPort         = 1602
	DNSListenAddr      = "127.0.0.42"
	DNSListenPort      = 53
	FakeIPRange        = "198.18.0.0/15"
	FakeIPTestDomain   = "fakeip.hybrid-failover"
	CheckProxyIPDomain = "ip.hybrid-failover"
	DirectTag          = "direct-out"
)

type OutboundKind string

const (
	OutboundDirect      OutboundKind = "direct"
	OutboundDirectBind  OutboundKind = "direct_bind"
	OutboundVLESS       OutboundKind = "vless"
	OutboundTrojan      OutboundKind = "trojan"
	OutboundShadowsocks OutboundKind = "shadowsocks"
	OutboundSocks       OutboundKind = "socks"
	OutboundHysteria2   OutboundKind = "hysteria2"
	OutboundAWG2Bind    OutboundKind = "awg2_bind"
	OutboundURLTest     OutboundKind = "urltest"
	OutboundSelector    OutboundKind = "selector"
)

type Plan struct {
	DNS          DNSPlan
	Sections     []SectionPlan
	Outbounds    []OutboundPlan
	Routes       []RouteRule
	RuleSets     []RuleSet
	ListDownload ListDownloadPlan
	OutputIface  string
}

type DNSPlan struct {
	Type         string
	Server       string
	Bootstrap    string
	RewriteTTL   int
	FakeIPRange  string
	FakeIPDomains []string
	RejectHTTPS  bool
}

type SectionPlan struct {
	Name           string
	ConnectionType string
	SelectorTag    string
	Enabled        bool
}

type OutboundPlan struct {
	Tag      string
	Kind     OutboundKind
	BindIface string
	ProxyURI string
	Members  []string
	Default  string
	URLTest  *URLTestPlan
}

type URLTestPlan struct {
	URL       string
	Interval  string
	Idle      string
	Tolerance int
	Interrupt bool
}

type RouteRule struct {
	Action      string
	OutboundTag string
	Section     string
	RuleSetTags []string
	Domains     []string
	DomainSuffix []string
	IPCIDR      []string
	Reject      bool
}

type RuleSet struct {
	Tag      string
	Kind     string // domains, subnets
	Domains  []string
	Subnets  []string
	Path     string
	RemoteURL string
}

type ListDownloadPlan struct {
	Enabled bool
	Section string
	Port    int
}

type ConnMeta struct {
	SrcIP    string
	SrcPort  int
	DstIP    string
	DstPort  int
	Domain   string
	Network  string
	Inbound  string
}

type DelaySample struct {
	Tag   string
	Delay time.Duration
	OK    bool
}

// Hash returns a stable SHA256 hex digest of the compiled plan.
func Hash(p *Plan) string {
	if p == nil {
		return ""
	}
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
