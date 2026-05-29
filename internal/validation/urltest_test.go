package validation

import "testing"

func TestSingboxDurationSeconds(t *testing.T) {
	cases := map[string]int{
		"30s": 30,
		"5m":  300,
		"1h":  3600,
	}
	for in, want := range cases {
		got, err := SingboxDurationSeconds(in)
		if err != nil || got != want {
			t.Fatalf("%s: got %d err=%v want %d", in, got, err, want)
		}
	}
}

func TestValidateURLTestDurationPairRejectsIntervalGreaterThanIdle(t *testing.T) {
	err := ValidateURLTestDurationPair("5m", "60s")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateURLTestDurationPairAcceptsValidPair(t *testing.T) {
	if err := ValidateURLTestDurationPair("30s", "5m"); err != nil {
		t.Fatal(err)
	}
}
