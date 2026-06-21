package diag

import (
	"testing"
)

func TestValidateFakeIPResult(t *testing.T) {
	if err := validateFakeIPResult("198.18.0.1"); err != nil {
		t.Fatalf("valid fakeip: %v", err)
	}
	if err := validateFakeIPResult(" 198.18.42.10 "); err != nil {
		t.Fatalf("trimmed fakeip: %v", err)
	}
	if err := validateFakeIPResult(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := validateFakeIPResult("8.8.8.8"); err == nil {
		t.Fatal("expected error for non-fakeip")
	}
}
