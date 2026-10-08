//go:build !with_utls

package outbound

import "testing"

// Without the with_utls tag VLESS flow xtls-rprx-vision cannot work over
// REALITY. Every build of the core uses the tag (scripts/build-packages.sh,
// the Go CI); a plain "go test" skips instead of failing on a missing tag.
func TestVisionNeedsWithUTLSTag(t *testing.T) {
	t.Skip("run with -tags with_utls to test VLESS Vision over REALITY")
}
