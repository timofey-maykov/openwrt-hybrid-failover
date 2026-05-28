package security

import "testing"

func TestAuthorizerAllowsAdmin(t *testing.T) {
	a := NewAuthorizer([]int64{1, 2, 3})
	if !a.IsAdmin(2) {
		t.Fatal("expected admin to be allowed")
	}
}

func TestAuthorizerDeniesUnknown(t *testing.T) {
	a := NewAuthorizer([]int64{1, 2, 3})
	if a.IsAdmin(99) {
		t.Fatal("expected unknown user to be denied")
	}
}
