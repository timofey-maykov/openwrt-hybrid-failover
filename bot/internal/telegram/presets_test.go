package telegram

import "testing"

func TestResolveParamKeyAlias(t *testing.T) {
	got := resolveParamKey("disable_quic")
	if got != "podkop.settings.disable_quic" {
		t.Fatalf("unexpected alias resolution: %s", got)
	}
}

func TestOnOffToBoolValue(t *testing.T) {
	v, err := onOffToBoolValue("on")
	if err != nil || v != "1" {
		t.Fatalf("unexpected on mapping: v=%s err=%v", v, err)
	}
	v, err = onOffToBoolValue("off")
	if err != nil || v != "0" {
		t.Fatalf("unexpected off mapping: v=%s err=%v", v, err)
	}
}
