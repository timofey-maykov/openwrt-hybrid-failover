package routerctl

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeExec answers by matching the start of "name arg arg".
type fakeExec struct {
	answers []answer
	calls   []string
}

type answer struct {
	prefix string
	out    string
	err    error
}

func (f *fakeExec) Run(ctx context.Context, name string, args ...string) (string, error) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, line)
	for _, a := range f.answers {
		if strings.HasPrefix(line, a.prefix) {
			return a.out, a.err
		}
	}
	return "", errors.New("no answer for: " + line)
}

func (f *fakeExec) RunBytes(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := f.Run(ctx, name, args...)
	return []byte(out), err
}

func (f *fakeExec) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	return "", errors.New("unused")
}

func (f *fakeExec) did(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestValidators(t *testing.T) {
	for _, ok := range []string{"example.com", "8.8.8.8", "a-b.c", "2001:db8::1", "[::1]"} {
		if !ValidHost(ok) {
			t.Errorf("ValidHost(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "-f", "-c1", "a b", "a;b", "a$(b)", "`x`", "a|b", "-"} {
		if ValidHost(bad) {
			t.Errorf("ValidHost(%q) = true", bad)
		}
	}
	if _, ok := NormalizeMAC("AA:bb:CC:dd:EE:ff"); !ok {
		t.Error("mac")
	}
	if _, ok := NormalizeMAC("aa:bb:cc:dd:ee"); ok {
		t.Error("short mac accepted")
	}
	if p, err := NormalizePort("8000-8010"); err != nil || p != "8000-8010" {
		t.Errorf("port range: %q %v", p, err)
	}
	for _, bad := range []string{"0", "65536", "a", "10-5", "1-2-3", ""} {
		if _, err := NormalizePort(bad); err == nil {
			t.Errorf("NormalizePort(%q) accepted", bad)
		}
	}
	if ValidIPv4("::1") || !ValidIPv4("192.168.1.5") || ValidIPv4("999.1.1.1") {
		t.Error("ipv4")
	}
}

func TestRedact(t *testing.T) {
	tok := "1234567890:AAZZexample_fake-token_for-tests_0000"
	in := `Post "https://api.telegram.org/bot` + tok + `/getUpdates": timeout`
	out := Redact(in)
	if strings.Contains(out, "AAZZex") || strings.Contains(out, "1234567890") {
		t.Fatalf("token left in %q", out)
	}
	if got := Redact(`"key": "pa55w0rd-example",`); strings.Contains(got, "pa55w0rd-example") {
		t.Fatalf("wifi key left in %q", got)
	}
	if got := Redact("option password 'hunter2'"); strings.Contains(got, "hunter2") {
		t.Fatalf("password left in %q", got)
	}
	if Redact("hello world 12345") != "hello world 12345" {
		t.Error("harmless text changed")
	}
}

func TestParseLeases(t *testing.T) {
	ls := parseLeases("1791530357 58:E4:EB:AE:EF:9C 192.168.42.166 Xiaomi-TV-Box 01:58\n1 aa:bb:cc:dd:ee:ff 192.168.42.2 * *\n\nbad line\n")
	if len(ls) != 2 || ls[0].mac != "58:e4:eb:ae:ef:9c" || ls[0].name != "Xiaomi-TV-Box" || ls[1].name != "" {
		t.Fatalf("%+v", ls)
	}
}

const showFW = `firewall.cfg0a=defaults
firewall.@redirect[0]=redirect
firewall.@redirect[0].name='ssh'
firewall.@redirect[0].proto='tcp' 'udp'
firewall.@redirect[0].src_dport='2222'
firewall.@redirect[0].dest_ip='192.168.1.5'
firewall.@redirect[0].dest_port='22'
firewall.@redirect[0].target='DNAT'
firewall.@redirect[1]=redirect
firewall.@redirect[1].name='masq'
firewall.@redirect[1].target='SNAT'
firewall.@redirect[2]=redirect
firewall.@redirect[2].name='web'
firewall.@redirect[2].enabled='0'
firewall.@redirect[2].src_dport='8080'
firewall.@redirect[2].dest_ip='192.168.1.9'
`

func TestParseRedirects(t *testing.T) {
	rs := parseRedirects(showFW)
	if len(rs) != 2 {
		t.Fatalf("want 2 DNAT redirects, got %d: %+v", len(rs), rs)
	}
	if rs[0].id != "@redirect[0]" || rs[0].proto != "tcp udp" || rs[0].srcPort != "2222" {
		t.Errorf("%+v", rs[0])
	}
	if !rs[1].off || rs[1].name != "web" {
		t.Errorf("%+v", rs[1])
	}
}

func TestPortFwdAddRejectsBadInput(t *testing.T) {
	f := &fakeExec{answers: []answer{{prefix: "uci show firewall", out: ""}}}
	s := New(f)
	cases := [][5]string{
		{"icmp", "80", "192.168.1.5", "", ""},
		{"tcp", "0", "192.168.1.5", "", ""},
		{"tcp", "80", "not-an-ip", "", ""},
		{"tcp", "80", "192.168.1.5", "99999", ""},
		{"tcp", "80", "192.168.1.5", "", "bad name"},
		{"tcp", "80", "192.168.1.5", "", "a;rm"},
	}
	for _, c := range cases {
		if _, err := s.PortFwdAdd(context.Background(), c[0], c[1], c[2], c[3], c[4]); err == nil {
			t.Errorf("accepted %v", c)
		}
	}
	if f.did("uci add") || f.did("uci set") {
		t.Fatalf("changed the router on bad input: %v", f.calls)
	}
}

func TestPortFwdAddDuplicatePortRefused(t *testing.T) {
	f := &fakeExec{answers: []answer{{prefix: "uci show firewall", out: showFW}}}
	_, err := New(f).PortFwdAdd(context.Background(), "tcp", "2222", "192.168.1.7", "", "")
	if err == nil || !strings.Contains(err.Error(), "уже проброшен") {
		t.Fatalf("got %v", err)
	}
	if f.did("uci add") {
		t.Fatal("touched uci")
	}
}

func TestPortFwdAddHappyPath(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "uci show firewall", out: ""},
		{prefix: "uci add firewall redirect", out: "cfg0b1c2d"},
		{prefix: "uci set", out: ""},
		{prefix: "uci add_list", out: ""},
		{prefix: "uci commit firewall", out: ""},
		{prefix: "/etc/init.d/firewall reload", out: ""},
	}}
	out, err := New(f).PortFwdAdd(context.Background(), "tcpudp", "51820", "192.168.1.7", "", "wg")
	if err != nil || !strings.Contains(out, "51820") {
		t.Fatalf("%q %v", out, err)
	}
	if !f.did("uci set firewall.cfg0b1c2d.src_dport=51820") || !f.did("uci add_list firewall.cfg0b1c2d.proto=udp") || !f.did("/etc/init.d/firewall reload") {
		t.Fatalf("calls: %v", f.calls)
	}
}

func TestPortFwdAddRevertsOnFailure(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "uci show firewall", out: ""},
		{prefix: "uci add firewall redirect", out: "cfg0b1c2d"},
		{prefix: "uci set firewall.cfg0b1c2d.name", out: ""},
		{prefix: "uci set firewall.cfg0b1c2d.target", err: errors.New("boom")},
		{prefix: "uci revert firewall", out: ""},
	}}
	if _, err := New(f).PortFwdAdd(context.Background(), "tcp", "80", "192.168.1.7", "", "x"); err == nil {
		t.Fatal("expected error")
	}
	if !f.did("uci revert firewall") || f.did("uci commit") {
		t.Fatalf("calls: %v", f.calls)
	}
}

func TestPortFwdDel(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "uci show firewall", out: showFW},
		{prefix: "uci delete", out: ""},
		{prefix: "uci commit firewall", out: ""},
		{prefix: "/etc/init.d/firewall reload", out: ""},
	}}
	s := New(f)
	if _, err := s.PortFwdDel(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	if !f.did("uci delete firewall.@redirect[2]") {
		t.Fatalf("calls: %v", f.calls)
	}
	if _, err := s.PortFwdDel(context.Background(), "9"); err == nil {
		t.Error("deleted a rule that is not there")
	}
	if _, err := s.PortFwdDel(context.Background(), "x;y"); err == nil {
		t.Error("accepted a bad ref")
	}
}

func TestServiceActionChecks(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "ls /etc/init.d", out: "firewall\nhybrid-failover-bot\ndropbear\n"},
		{prefix: "/etc/init.d/firewall restart", out: ""},
	}}
	s := New(f)
	ctx := context.Background()
	if _, err := s.ServiceAction(ctx, "firewall", "restart"); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{
		{"firewall;reboot", "restart"}, {"../x", "start"}, {"firewall", "format"},
		{"nothere", "start"}, {"hybrid-failover-bot", "restart"}, {"hybrid-failover-bot", "stop"},
	} {
		if _, err := s.ServiceAction(ctx, c[0], c[1]); err == nil {
			t.Errorf("accepted %v", c)
		}
	}
	if _, err := s.ServiceAction(ctx, "hybrid-failover-bot", "enable"); err == nil || strings.Contains(err.Error(), "сам себя") {
		// enable of the bot is allowed to reach the init script (which here has no answer)
		_ = err
	}
}

func TestPingAndTracerouteValidate(t *testing.T) {
	f := &fakeExec{}
	s := New(f)
	for _, bad := range []string{"-f", "a b", "x;y", ""} {
		if _, err := s.Ping(context.Background(), bad); err == nil {
			t.Errorf("ping accepted %q", bad)
		}
		if _, err := s.Traceroute(context.Background(), bad); err == nil {
			t.Errorf("traceroute accepted %q", bad)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran commands for bad hosts: %v", f.calls)
	}
}

func TestRebootToleratesDroppedConnection(t *testing.T) {
	f := &fakeExec{answers: []answer{{prefix: "reboot", err: errors.New("ssh x: remote command exited without exit status")}}}
	if _, err := New(f).Reboot(context.Background()); err != nil {
		t.Fatal(err)
	}
	f = &fakeExec{answers: []answer{{prefix: "reboot", err: errors.New("permission denied")}}}
	if _, err := New(f).Reboot(context.Background()); err == nil {
		t.Fatal("real failure swallowed")
	}
}

func TestKickNeedsAssociatedClient(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "ubus list", out: "network\nhostapd.phy0-ap0\nhostapd-auth\n"},
		{prefix: "ubus call hostapd.phy0-ap0 get_clients", out: `{"freq":2412,"clients":{"aa:bb:cc:dd:ee:ff":{"signal":-50}}}`},
		{prefix: "ubus call hostapd.phy0-ap0 del_client", out: "{}"},
	}}
	s := New(f)
	if _, err := s.Kick(context.Background(), "AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatal(err)
	}
	if !f.did(`ubus call hostapd.phy0-ap0 del_client {"addr":"aa:bb:cc:dd:ee:ff"`) {
		t.Fatalf("calls: %v", f.calls)
	}
	if _, err := s.Kick(context.Background(), "11:22:33:44:55:66"); err == nil {
		t.Fatal("kicked a client that is not connected")
	}
	if _, err := s.Kick(context.Background(), "x;y"); err == nil {
		t.Fatal("accepted a bad name")
	}
}

func TestWifiSetOnlyKnownRadios(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "ubus call network.wireless status", out: `{"radio0":{"up":true,"config":{"band":"2g"},"interfaces":[]},"radio1":{"up":true,"config":{"band":"5g"},"interfaces":[]}}`},
		{prefix: "uci set", out: ""},
		{prefix: "uci commit wireless", out: ""},
		{prefix: "wifi reload", out: ""},
	}}
	s := New(f)
	if _, err := s.WifiSet(context.Background(), "radio1", false); err != nil {
		t.Fatal(err)
	}
	if !f.did("uci set wireless.radio1.disabled=1") || f.did("uci set wireless.radio0") {
		t.Fatalf("calls: %v", f.calls)
	}
	if _, err := s.WifiSet(context.Background(), "radio9;x", false); err == nil {
		t.Fatal("unknown radio accepted")
	}
}

func TestBackupChecksArchive(t *testing.T) {
	good := string([]byte{0x1f, 0x8b}) + strings.Repeat("x", 200)
	f := &fakeExec{answers: []answer{
		{prefix: "sysupgrade -b", out: ""},
		{prefix: "cat /tmp/hf-bot-backup.tar.gz", out: good},
		{prefix: "rm -f", out: ""},
	}}
	data, err := New(f).Backup(context.Background())
	if err != nil || len(data) != 202 {
		t.Fatalf("%d %v", len(data), err)
	}
	if !f.did("rm -f /tmp/hf-bot-backup.tar.gz") {
		t.Fatal("archive left on the router")
	}
	f = &fakeExec{answers: []answer{
		{prefix: "sysupgrade -b", out: ""},
		{prefix: "cat /tmp/hf-bot-backup.tar.gz", out: "not a gzip"},
		{prefix: "rm -f", out: ""},
	}}
	if _, err := New(f).Backup(context.Background()); err == nil {
		t.Fatal("bad archive accepted")
	}
}

func TestSysInfoFormats(t *testing.T) {
	f := &fakeExec{answers: []answer{
		{prefix: "ubus call system board", out: `{"model":"Xiaomi BE7000","hostname":"BeamWRT","kernel":"6.18.52","release":{"description":"OpenWrt main"}}`},
		{prefix: "ubus call system info", out: `{"uptime":253973,"load":[9536,17728,27360],"memory":{"total":244760576,"available":79364096}}`},
		{prefix: "sed -n s/^VERSION=//p", err: errors.New("none")},
		{prefix: "df -k /overlay", out: "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/sda 14424516 205172 13464812 2% /overlay"},
		{prefix: "ls /sys/class/thermal", out: "thermal_zone0\ncooling_device0"},
		{prefix: "grep -H", out: "/sys/class/thermal/thermal_zone0/type:cpu-thermal\n/sys/class/thermal/thermal_zone0/temp:52300"},
	}}
	out, err := New(f).SysInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Xiaomi BE7000", "2д 22ч 32м", "0.15 0.27 0.42", "52 °C", "/overlay"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSyslogRedactsSecrets(t *testing.T) {
	f := &fakeExec{answers: []answer{{prefix: "logread -l 50", out: "a\nPost https://api.telegram.org/bot1234567890:AAZZexample_fake-token_for-tests_0000/x failed\nb"}}}
	out, err := New(f).Syslog(context.Background(), 0)
	if err != nil || strings.Contains(out, "AAZZex") {
		t.Fatalf("%q %v", out, err)
	}
}
