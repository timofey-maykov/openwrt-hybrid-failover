package lifecycle

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
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

func TestAWG2RandomPortDiffersFromOld(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := awg2RandomPort("20000")
		n, err := strconv.Atoi(p)
		if err != nil || n < 20000 || n > 59999 || p == "20000" {
			t.Fatalf("bad port %q", p)
		}
	}
}

func TestOrphanPawgNames(t *testing.T) {
	keep := map[string]struct{}{
		"pawgkeep1": {},
		"pawgkeep2": {},
	}
	present := []string{"pawgkeep1", "pawgold", "pawgkeep2", "pawgdrop"}
	got := orphanPawgNames(present, keep)
	want := []string{"pawgold", "pawgdrop"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if leftover := orphanPawgNames([]string{"pawgkeep1", "pawgkeep2"}, keep); len(leftover) != 0 {
		t.Fatalf("kept ifaces must not be orphan: %v", leftover)
	}
}

func TestExpectedAWG2IfnamesDropsRemovedPeers(t *testing.T) {
	linkA := "awg2://192.0.2.10:41490?address=10.8.1.2%2F32&private_key=ka&public_key=pa"
	linkB := "awg2://192.0.2.11:41491?address=10.8.1.3%2F32&private_key=kb&public_key=pb"
	linkC := "awg2://192.0.2.12:41492?address=10.8.1.4%2F32&private_key=kc&public_key=pc"

	pkgThree, err := uci.Parse("config section 'glob'\n\toption connection_type 'proxy'\n\toption proxy_config_type 'urltest'\n" +
		"\tlist urltest_proxy_links '" + linkA + "'\n" +
		"\tlist urltest_proxy_links '" + linkB + "'\n" +
		"\tlist urltest_proxy_links '" + linkC + "'\n")
	if err != nil {
		t.Fatal(err)
	}
	keepThree, err := expectedAWG2Ifnames(pkgThree)
	if err != nil {
		t.Fatal(err)
	}
	if len(keepThree) != 3 {
		t.Fatalf("three distinct peers → three ifaces, got %v", keepThree)
	}

	pkgOne, err := uci.Parse("config section 'glob'\n\toption connection_type 'proxy'\n\toption proxy_config_type 'urltest'\n" +
		"\tlist urltest_proxy_links '" + linkA + "'\n")
	if err != nil {
		t.Fatal(err)
	}
	keepOne, err := expectedAWG2Ifnames(pkgOne)
	if err != nil {
		t.Fatal(err)
	}
	if len(keepOne) != 1 {
		t.Fatalf("one peer → one iface, got %v", keepOne)
	}
	for name := range keepThree {
		if _, ok := keepOne[name]; ok {
			continue
		}
		// B and C must become orphans relative to the shrunk config.
		if orphans := orphanPawgNames([]string{name}, keepOne); len(orphans) != 1 {
			t.Fatalf("%s should be orphan after shrink, keepOne=%v", name, keepOne)
		}
	}
}

func TestExpectedAWG2IfnamesSamePeerOneIface(t *testing.T) {
	// Same public key, two endpoints: one shared interface.
	link1 := "awg2://192.0.2.10:41490?address=10.8.1.2%2F32&private_key=k&public_key=SAMEPEER"
	link2 := "awg2://198.51.100.10:41490?address=10.8.1.2%2F32&private_key=k&public_key=SAMEPEER"
	pkg, err := uci.Parse("config section 'glob'\n\toption connection_type 'proxy'\n\toption proxy_config_type 'urltest'\n" +
		"\tlist urltest_proxy_links '" + link1 + "'\n" +
		"\tlist urltest_proxy_links '" + link2 + "'\n")
	if err != nil {
		t.Fatal(err)
	}
	keep, err := expectedAWG2Ifnames(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 1 {
		t.Fatalf("same peer → one iface, got %v", keep)
	}
}

func TestRemoveAWG2EndpointsFile(t *testing.T) {
	dir := t.TempDir()
	old := paths.AWG2EndpointsDir
	paths.AWG2EndpointsDir = dir
	defer func() { paths.AWG2EndpointsDir = old }()

	writeAWG2Endpoints("pawgorphan", []string{"203.0.113.10:1"})
	removeAWG2Endpoints("pawgorphan")
	if _, err := os.Stat(filepath.Join(dir, "pawgorphan")); !os.IsNotExist(err) {
		t.Fatalf("endpoints file must be gone, err=%v", err)
	}
}

func TestMissingAddresses(t *testing.T) {
	const withAddr = "12: pawg1    inet 10.78.0.2/32 scope global pawg1\\       valid_lft forever preferred_lft forever\n"
	const without = "12: pawg1: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1280 qdisc noqueue state UNKNOWN\\    link/none\n"

	if got := missingAddresses(withAddr, "10.78.0.2/32"); len(got) != 0 {
		t.Fatalf("address is there, got missing %v", got)
	}
	if got := missingAddresses(without, "10.78.0.2/32"); len(got) != 1 || got[0] != "10.78.0.2/32" {
		t.Fatalf("got %v", got)
	}
	if got := missingAddresses(withAddr, "10.78.0.2/32, fd00::2/128"); len(got) != 1 || got[0] != "fd00::2/128" {
		t.Fatalf("only the second address is missing, got %v", got)
	}
	if got := missingAddresses(without, ""); len(got) != 0 {
		t.Fatalf("nothing wanted, got %v", got)
	}
}
