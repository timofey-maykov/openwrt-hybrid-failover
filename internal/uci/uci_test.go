package uci

import (
	"strings"
	"testing"
)

func TestParseMultilineOption(t *testing.T) {
	pkg, err := Parse(`
config section 'glob'
	option user_subnet_list_type 'text'
	option user_subnets_text '102.132.96.0/20
149.154.160.0/20
91.108.4.0/22'
`)
	if err != nil {
		t.Fatal(err)
	}
	sec := pkg.Section("glob")
	if sec == nil {
		t.Fatal("missing glob section")
	}
	got := sec.Get("user_subnets_text", "")
	if got == "" {
		t.Fatal("empty user_subnets_text")
	}
	for _, want := range []string{"102.132.96.0/20", "149.154.160.0/20", "91.108.4.0/22"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in multiline value: %q", want, got)
		}
	}
}
