package subscription

import (
	"strings"
	"testing"
)

func TestCronJobForInterval(t *testing.T) {
	for in, want := range map[string]string{
		"1h":  "37 * * * * ",
		"3h":  "37 */3 * * * ",
		"6h":  "37 */6 * * * ",
		"12h": "37 */12 * * * ",
		"1d":  "37 9 * * * ",
	} {
		job, on, ok := cronJobForInterval(in)
		if !ok || !on || !strings.HasPrefix(job, want) || !strings.HasSuffix(job, refreshCronMarker) {
			t.Errorf("%s: job=%q on=%v ok=%v", in, job, on, ok)
		}
	}
}

func TestCronJobForInterval_OffAndInvalid(t *testing.T) {
	for _, in := range []string{"", "off", "0"} {
		if job, on, ok := cronJobForInterval(in); job != "" || on || !ok {
			t.Errorf("%q: job=%q on=%v ok=%v, want off", in, job, on, ok)
		}
	}
	for _, in := range []string{"2h", "5m", "weekly"} {
		if _, _, ok := cronJobForInterval(in); ok {
			t.Errorf("%q should be rejected", in)
		}
	}
}
