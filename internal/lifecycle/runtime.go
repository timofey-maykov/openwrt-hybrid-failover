package lifecycle

import "context"

var bgCancel context.CancelFunc

// StartBackground runs failover policy controller and sing-box watchdog until CancelBackground.
func StartBackground(uciPath string) {
	CancelBackground()
	ctx, cancel := context.WithCancel(context.Background())
	bgCancel = cancel
	go DefaultFailoverController(uciPath).Run(ctx)
	go DefaultWatchdog().Run(ctx)
}

// CancelBackground stops background controller and watchdog goroutines.
func CancelBackground() {
	if bgCancel != nil {
		bgCancel()
		bgCancel = nil
	}
}
