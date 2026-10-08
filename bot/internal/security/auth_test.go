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

func TestViewerSeesRouterStateButChangesNothing(t *testing.T) {
	a := NewAuthorizer([]int64{1}, []int64{2})
	read := []string{"/sysinfo", "/wan", "/devices", "/wifi", "/syslog 50", "/dmesg", "/services", "/portfwd", "/slots"}
	for _, c := range read {
		if !a.Allowed(2, c) {
			t.Errorf("viewer denied %q", c)
		}
	}
	write := []string{
		"/reboot", "/service dropbear stop", "/ifdown wan", "/ifup wan", "/wifi_on", "/wifi_off", "/kick aa:bb:cc:dd:ee:ff",
		"/wol pc", "/portfwd_add tcp 80 192.168.1.2", "/portfwd_del 1", "/fw_restart", "/apk_check", "/apk_upgrade",
		"/backup", "/update_check", "/update_apply", "/mode5g mlo", "/sh id", "/ping 8.8.8.8", "/traceroute 8.8.8.8",
	}
	for _, c := range write {
		if a.Allowed(2, c) {
			t.Errorf("viewer allowed %q", c)
		}
		if !a.Allowed(1, c) {
			t.Errorf("admin denied %q", c)
		}
	}
	for _, c := range append(read, write...) {
		if a.Allowed(3, c) {
			t.Errorf("stranger allowed %q", c)
		}
	}
}
