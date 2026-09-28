package security

import "testing"

func TestAuthorizerAllowsAdmin(t *testing.T) {
	a := NewAuthorizer([]int64{1, 2, 3}, nil)
	if !a.IsAdmin(2) {
		t.Fatal("expected admin to be allowed")
	}
}

func TestAuthorizerDeniesUnknown(t *testing.T) {
	a := NewAuthorizer([]int64{1, 2, 3}, nil)
	if a.IsAdmin(99) {
		t.Fatal("expected unknown user to be denied")
	}
}

func TestViewerCanSelectRouter(t *testing.T) {
	a := NewAuthorizer([]int64{1}, []int64{2})
	for _, cmd := range []string{"/routers", "/use office", "/router", "/status"} {
		if !a.Allowed(2, cmd) {
			t.Fatalf("viewer denied %q", cmd)
		}
	}
	if a.Allowed(2, "/param_apply") {
		t.Fatal("viewer allowed to apply")
	}
}
