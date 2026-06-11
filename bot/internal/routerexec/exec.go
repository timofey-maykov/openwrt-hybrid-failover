package routerexec

import "context"

// Exec runs commands on a hybrid-failover host (local OpenWrt or remote via SSH).
type Exec interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
	RunCoreRPC(ctx context.Context, method string, args ...string) (string, error)
}
