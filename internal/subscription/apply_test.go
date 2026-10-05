package subscription

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func TestOwnedLinksRoundTrip(t *testing.T) {
	old := paths.SubscriptionState
	paths.SubscriptionState = filepath.Join(t.TempDir(), "sub", "links.json")
	t.Cleanup(func() { paths.SubscriptionState = old })

	if got := loadOwned("glob"); got != nil {
		t.Fatalf("no state yet, got %v", got)
	}
	saveOwned("glob", []string{"awg2://a", "awg2://b"})
	if got := loadOwned("glob"); !reflect.DeepEqual(got, []string{"awg2://a", "awg2://b"}) {
		t.Fatalf("loadOwned = %v", got)
	}
	if got := loadOwned("other"); got != nil {
		t.Fatalf("state of another section must not apply, got %v", got)
	}
}

func TestRefreshReplacesOnlySubscriptionLinks(t *testing.T) {
	existing := []string{"hy2://manual", "awg2://old-server", "awg2://kept"}
	owned := []string{"awg2://old-server", "awg2://kept"}
	incoming := []string{"awg2://kept", "awg2://new-server"}

	got := mergeLinks(dropOwned(existing, owned), incoming)
	want := []string{"hy2://manual", "awg2://kept", "awg2://new-server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDropOwnedWithoutState(t *testing.T) {
	existing := []string{"hy2://manual"}
	if got := dropOwned(existing, nil); !reflect.DeepEqual(got, existing) {
		t.Fatalf("got %v", got)
	}
}
