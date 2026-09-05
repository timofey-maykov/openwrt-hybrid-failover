package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func TestWriteAWG2ConfigAWG31(t *testing.T) {
	path, err := writeAWG2Config(amnezia.AWG2Params{
		Host:                   "192.0.2.10",
		Port:                   "41490",
		PrivateKey:             "client-priv",
		PublicKey:              "server-pub",
		PresharedKey:           "psk",
		AllowedIPs:             "0.0.0.0/0,::/0",
		PersistentKeepalive:    "25-35",
		Jc:                     "4",
		S1:                     "67",
		S4:                     "12",
		HeaderProtectionKey:    "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD=",
		ContentPaddingAddition: "10-100",
		RandomTrailers:         "on",
		DisableCookies:         "on",
		RekeyAfterTime:         "100-120",
		RekeyTimeout:           "3-7",
		RejectAfterTime:        "150-180",
		KeepaliveTimeout:       "5-15",
		MaxHandshakeAttempts:   "15-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"HeaderProtectionKey = DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD=",
		"ContentPaddingAddition = 10-100",
		"RandomTrailers = on",
		"DisableCookies = on",
		"RekeyAfterTime = 100-120",
		"RekeyTimeout = 3-7",
		"RejectAfterTime = 150-180",
		"KeepaliveTimeout = 5-15",
		"MaxHandshakeAttempts = 15-20",
		"PersistentKeepalive = 25-35",
		"S4 = 12",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in conf:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Address =") {
		t.Fatalf("setconf must not include Address:\n%s", text)
	}
}

func TestWrapAWGSetConfRandomTrailers(t *testing.T) {
	err := wrapAWGSetConf(os.ErrInvalid, []byte("Line unrecognized: `RandomTrailers=on'\nConfiguration parsing error"))
	if err == nil || !strings.Contains(err.Error(), "3.1+") {
		t.Fatalf("err=%v", err)
	}
}

func TestNextAWG2Endpoint(t *testing.T) {
	cases := []struct {
		current string
		list    []string
		want    string
	}{
		{"203.0.113.10:38889", []string{"203.0.113.10:38889", "198.51.100.10:38889"}, "198.51.100.10:38889"},
		{"198.51.100.10:38889", []string{"203.0.113.10:38889", "198.51.100.10:38889"}, "203.0.113.10:38889"},
		{"1.2.3.4:1", []string{"203.0.113.10:38889", "198.51.100.10:38889"}, "203.0.113.10:38889"},
	}
	for _, tc := range cases {
		got := nextAWG2Endpoint(tc.current, tc.list)
		if got != tc.want {
			t.Fatalf("current=%q list=%v got=%q want=%q", tc.current, tc.list, got, tc.want)
		}
	}
	if nextAWG2Endpoint("a", []string{"a"}) != "" {
		t.Fatal("single endpoint must not rotate")
	}
}

func TestWriteReadAWG2Endpoints(t *testing.T) {
	dir := t.TempDir()
	old := paths.AWG2EndpointsDir
	paths.AWG2EndpointsDir = dir
	defer func() { paths.AWG2EndpointsDir = old }()

	writeAWG2Endpoints("pawgtest", []string{"203.0.113.10:38889", "198.51.100.10:38889", "203.0.113.10:38889"})
	got := readAWG2Endpoints("pawgtest")
	if len(got) != 2 || got[0] != "203.0.113.10:38889" || got[1] != "198.51.100.10:38889" {
		t.Fatalf("got=%v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "pawgtest")); err != nil {
		t.Fatal(err)
	}
}

func TestParseAWGShowPeer(t *testing.T) {
	text := `interface: pawgtest0001
  public key: BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=
  private key: (hidden)
peer: CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=
  endpoint: 192.0.2.10:38889
  latest handshake: 31 seconds ago`

	endpoint, publicKey, ok := parseAWGShowPeer(text)
	if !ok {
		t.Fatal("expected peer parse ok")
	}
	if endpoint != "192.0.2.10:38889" {
		t.Fatalf("endpoint=%q", endpoint)
	}
	if publicKey != "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=" {
		t.Fatalf("publicKey=%q", publicKey)
	}
}

func TestParseAWGShowPeerMissing(t *testing.T) {
	if _, _, ok := parseAWGShowPeer("interface: pawg\n  public key: x"); ok {
		t.Fatal("expected missing peer")
	}
}
