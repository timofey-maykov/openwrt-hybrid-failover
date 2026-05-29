package amnezia

import "testing"

func TestDescribeLinkVLESS(t *testing.T) {
	s, err := DescribeLink("vless://uuid@1.2.3.4:443?security=reality")
	if err != nil {
		t.Fatal(err)
	}
	if s == "" {
		t.Fatal("empty summary")
	}
}

func TestDescribeLinkEmpty(t *testing.T) {
	_, err := DescribeLink("")
	if err == nil {
		t.Fatal("expected error")
	}
}
