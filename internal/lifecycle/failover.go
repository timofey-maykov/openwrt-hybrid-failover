package lifecycle

import (
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/failover"
)

// DefaultFailoverController returns the policy-aware failover controller.
func DefaultFailoverController(uciPath string) *failover.Controller {
	c := failover.DefaultController(uciPath)
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	return c
}
