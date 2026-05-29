package singbox

const (
	FAKEIPTestDomain     = "fakeip.hybrid-failover"
	CheckProxyIPDomain   = "ip.hybrid-failover"
	FakeIPInet4Range     = "198.18.0.0/15"
	FakeIPTestPort       = 8443
	DefaultDNSRewriteTTL = "60"

	GitHubRawURL = "https://raw.githubusercontent.com/itdoginfo/allow-domains/main"
	SRSMainURL   = "https://github.com/itdoginfo/allow-domains/releases/latest/download"

	RulesetDir = "/tmp/hybrid-failover/rulesets"

	DirectTag           = "direct-out"
	TPROXYInboundTag    = "tproxy-in"
	DNSInboundTag       = "dns-in"
	DNSInboundAddress   = "127.0.0.42"
	DNSInboundPort      = 53
	DNSServerTag        = "dns-server"
	BootstrapTag        = "bootstrap-dns-server"
	FakeIPTag           = "fakeip-server"
	FakeIPDNSRuleTag    = "fakeip-dns-rule-tag"
	RejectRuleTag       = "reject-rule-tag"
	ClashAPIPort        = 9090
	DefaultClashListen  = "127.0.0.1:9090"
	DefaultCachePath    = "/etc/sing-box/cache.db"
	DefaultBootstrapDNS = "77.88.8.8"
	DefaultDNSServer    = "1.1.1.1"
	DefaultDNSType      = "doh"
	DefaultUpdateInterval = "1d"
)

var CommunityServices = []string{
	"russia_inside", "russia_outside", "ukraine_inside", "geoblock", "block", "porn",
	"news", "anime", "youtube", "hdrezka", "tiktok", "google_ai", "google_play",
	"hodca", "discord", "meta", "twitter", "cloudflare", "cloudfront", "digitalocean",
	"hetzner", "ovh", "telegram", "roblox", "netflix",
}

var SubnetListURLs = map[string]string{
	"twitter":       GitHubRawURL + "/Subnets/IPv4/twitter.lst",
	"meta":          GitHubRawURL + "/Subnets/IPv4/meta.lst",
	"discord":       GitHubRawURL + "/Subnets/IPv4/discord.lst",
	"roblox":        GitHubRawURL + "/Subnets/IPv4/roblox.lst",
	"telegram":      GitHubRawURL + "/Subnets/IPv4/telegram.lst",
	"cloudflare":    GitHubRawURL + "/Subnets/IPv4/cloudflare.lst",
	"hetzner":       GitHubRawURL + "/Subnets/IPv4/hetzner.lst",
	"ovh":           GitHubRawURL + "/Subnets/IPv4/ovh.lst",
	"digitalocean":  GitHubRawURL + "/Subnets/IPv4/digitalocean.lst",
	"cloudfront":    GitHubRawURL + "/Subnets/IPv4/cloudfront.lst",
}

func CommunityServiceDomainURL(service string) string {
	return GitHubRawURL + "/Services/" + service
}

func CommunitySRSURL(service string) string {
	return SRSMainURL + "/" + service + ".srs"
}
