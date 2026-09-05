package netlink

import "testing"

func TestPickDefaultRouteCandidatePrefersPPPoE(t *testing.T) {
	raw := []byte(`{
  "interface": [
    {
      "interface": "wan",
      "up": true,
      "l3_device": "wan",
      "metric": 0,
      "route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "10.213.1.254"}]
    },
    {
      "interface": "vpnl2tp",
      "up": true,
      "l3_device": "pppoe-vpnl2tp",
      "metric": 0,
      "route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "78.25.155.117"}]
    }
  ]
}`)
	c, ok := pickDefaultRouteCandidate(raw)
	if !ok {
		t.Fatal("expected candidate")
	}
	if c.dev != "pppoe-vpnl2tp" || c.nexthop != "78.25.155.117" {
		t.Fatalf("got %+v", c)
	}
}

func TestPickDefaultRouteCandidateSkipsDown(t *testing.T) {
	raw := []byte(`{
  "interface": [
    {
      "interface": "vpnl2tp",
      "up": false,
      "l3_device": "pppoe-vpnl2tp",
      "metric": 0,
      "route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "78.25.155.117"}]
    },
    {
      "interface": "wan",
      "up": true,
      "l3_device": "wan",
      "metric": 10,
      "route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "10.213.1.254"}]
    }
  ]
}`)
	c, ok := pickDefaultRouteCandidate(raw)
	if !ok {
		t.Fatal("expected fallback candidate")
	}
	if c.dev != "wan" || c.nexthop != "10.213.1.254" {
		t.Fatalf("got %+v", c)
	}
}
