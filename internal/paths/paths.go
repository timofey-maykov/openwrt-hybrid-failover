package paths

// Legacy* are on-disk paths from pre-hybrid-failover installs (used only by migrate).
const (
	UCIPackage           = "hybrid-failover"
	UCIConfig            = "/etc/config/hybrid-failover"
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
	FailoverStateFile  = "/var/run/hybrid-failover/policy-state.json"
	MetricsPromFile    = "/var/run/hybrid-failover/metrics.prom"
	AuditFile          = "/var/log/hybrid-failover/audit.log"
	ListUpdatePID      = "/var/run/hybrid-failover-list-update.pid"
	DefaultMainSection = "glob"
)

const legacyPkg = "podkop"
