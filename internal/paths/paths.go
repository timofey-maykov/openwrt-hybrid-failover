package paths

// Runtime paths (vars allow tests to redirect).
var (
	UCIConfig         = "/etc/config/hybrid-failover"
	FailoverStateFile = "/var/run/hybrid-failover/policy-state.json"
	MetricsPromFile   = "/var/run/hybrid-failover/metrics.prom"
	DelayHistoryFile  = "/var/run/hybrid-failover/delay-history.json"
	EngineRuntimeFile = "/var/run/hybrid-failover/engine-runtime.json"
	SwitchRequestFile = "/var/run/hybrid-failover/switch-request.json"
	SwitchResponseFile = "/var/run/hybrid-failover/switch-response.json"
)

// Legacy* are on-disk paths from pre-hybrid-failover installs (used only by migrate).
const (
	UCIPackage           = "hybrid-failover"
	LegacyUCIConfig      = "/etc/config/" + legacyPkg
	LegacyRoutingBinary  = "/usr/bin/" + legacyPkg
	LegacyFailoverHook   = "/usr/bin/" + legacyPkg + "-failover-apply.sh"
	LegacyInitScript     = "/etc/init.d/" + legacyPkg
	SingboxConfig      = "/etc/sing-box/config.json"
	SingboxCache       = "/etc/sing-box/cache.db"
	SingboxInit        = "/etc/init.d/sing-box"
	CoreInit           = "/etc/init.d/hybrid-failover"
	PendingDir         = "/etc/hybrid-failover/pending"
	HistoryFile        = "/var/log/hybrid-failover/history.jsonl"
	AuditFile          = "/var/log/hybrid-failover/audit.log"
	ListUpdatePID      = "/var/run/hybrid-failover-list-update.pid"
	DefaultMainSection = "glob"
)

const legacyPkg = "podkop"
