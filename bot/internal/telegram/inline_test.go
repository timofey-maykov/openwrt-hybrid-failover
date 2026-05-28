package telegram

import "testing"

func TestCallbackToCommand(t *testing.T) {
	cmd, ok := callbackToCommand("cmd:/status")
	if !ok {
		t.Fatal("expected callback mapping to succeed")
	}
	if cmd != "/status" {
		t.Fatalf("unexpected command: %s", cmd)
	}
}

func TestCallbackToCommandRejectsInvalidPayload(t *testing.T) {
	if _, ok := callbackToCommand("status"); ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}

func TestCallbackToNav(t *testing.T) {
	nav, ok := callbackToNav("nav:params")
	if !ok || nav != "params" {
		t.Fatalf("unexpected nav parse: ok=%v nav=%q", ok, nav)
	}
}

func TestCallbackToConfirm(t *testing.T) {
	cmd, ok := callbackToConfirm("confirm:/param_apply")
	if !ok || cmd != "/param_apply" {
		t.Fatalf("unexpected confirm parse: ok=%v cmd=%q", ok, cmd)
	}
}
