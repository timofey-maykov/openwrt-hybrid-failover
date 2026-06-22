package diag

import (
	"fmt"
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

func TestIsDNSTransient(t *testing.T) {
	if isDNSTransient(fmt.Errorf("read udp: connection refused")) != true {
		t.Fatal("connection refused should be transient")
	}
	if isDNSTransient(fmt.Errorf("permanent failure")) != false {
		t.Fatal("unexpected transient")
	}
}
