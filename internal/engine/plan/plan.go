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
	// OutboundFallback dials the first live member in order (a list bound to
	// one channel, then its fallback: the section pool, direct, or nothing).
	OutboundFallback OutboundKind = "fallback"
	// OutboundBalance spreads connections over the live members, the same
	// site (registrable domain) always on the same member while it lives.
	OutboundBalance OutboundKind = "balance"
)

type Plan struct {
	DNS          DNSPlan
	Sections     []SectionPlan
	Outbounds    []OutboundPlan
	Routes       []RouteRule
	RuleSets     []RuleSet
	ListDownload ListDownloadPlan
	OutputIface  string
	DisableQUIC  bool
	// Channels lists the channels of each section, for bindings and status.
	Channels []ChannelPlan `json:",omitempty"`
	// Bindings are the per-list channel bindings, in rule order.
	Bindings []BindingPlan `json:",omitempty"`
}

// ChannelPlan is one channel of a section: a stable id and its outbound tag.
type ChannelPlan struct {
	Section string
	ID      string
	Name    string
	Tag     string
}

// BindingPlan is one list_route as compiled.
type BindingPlan struct {
	Name        string
	Section     string
	Lists       []string
	Channel     string // channel id, balance, direct or block
	ChannelTag  string // outbound of the bound channel, empty for balance/direct/block
	OnDown      string
	OutboundTag string // what the rule dials: fallback/balance group, direct-out, or empty for block
	// Missing is set when the bound channel id is not in the section any
	// more; the lists then stay with the section pool.
	Missing bool `json:",omitempty"`
}

type DNSPlan struct {
	Type          string
	Server        string
	Bootstrap     string
	RewriteTTL    int
	FakeIPRange   string
	FakeIPDomains []string
	RejectHTTPS   bool
}

type SectionPlan struct {
	Name           string
	ConnectionType string
	SelectorTag    string
	Enabled        bool
	// ListBased means unmatched traffic must stay direct (sing-box final), not catch-all to VPN/proxy.
	ListBased bool
}

type OutboundPlan struct {
	Tag       string
	Kind      OutboundKind
	BindIface string
	ProxyURI  string
	Members   []string
	Default   string
	URLTest   *URLTestPlan
}

type URLTestPlan struct {
	URL       string
	Interval  string
	Idle      string
	Tolerance int
	Interrupt bool
}

type RouteRule struct {
	Action       string
	OutboundTag  string
	Section      string
	RuleSetTags  []string
	Domains      []string
	DomainSuffix []string
	IPCIDR       []string
	SourceIPCIDR []string
	// Network limits the rule to "tcp" or "udp"; empty matches both.
	Network string
	Reject  bool
	// Binding is the list_route name for rules of a per-list binding.
	Binding string `json:",omitempty"`
}

type RuleSet struct {
	Tag       string
	Kind      string // domains, subnets
	Domains   []string
	Subnets   []string
	Path      string
	RemoteURL string
	// FileStamp is mtime:size of Path so Hash() changes when list files are filled.
	FileStamp string
}

type ListDownloadPlan struct {
	Enabled bool
	Section string
	Port    int
}

type ConnMeta struct {
	SrcIP   string
	SrcPort int
	DstIP   string
	DstPort int
	Domain  string
	Network string
	Inbound string
}

type DelaySample struct {
	Tag   string
	Delay time.Duration
	OK    bool
	Error string
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
