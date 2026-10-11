package subscription

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

// panelServer answers like a subscription panel: amneziawg:// lines with
// named servers and the profile headers.
func panelServer(t *testing.T, names []string, expire int64, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("Profile-Title", "PulseVPN 3")
		h.Set("Support-Url", "https://t.me/example_bot?start=support")
		if expire > 0 {
			h.Set("Subscription-Userinfo", "upload=0; download=0; total=0; expire="+itoa(expire))
		}
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("Подписка не активна\nПродлить её можно в боте."))
			return
		}
		for i, name := range names {
			conf := strings.Replace(subConf, "Endpoint = 192.0.2.10:41491", "Endpoint = 192.0.2."+itoa(int64(10+i))+":41491", 1)
			conf = strings.Replace(conf, "PublicKey = c2VydmVy", "PublicKey = c2VydmVy"+itoa(int64(i)), 1)
			_, _ = w.Write([]byte("amneziawg://" + base64.RawURLEncoding.EncodeToString([]byte(conf)) + "#" + url.PathEscape(name) + "\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestFetchKeepsServerNamesAndPanelHeaders(t *testing.T) {
	expire := time.Now().Add(400 * 24 * time.Hour).Unix()
	u := panelServer(t, []string{"Латвия, Рига", "США, Канзас-Сити 2"}, expire, 200)

	res, err := NewFetcher().Fetch([]string{u})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Links) != 2 || res.Links[0].Name != "Латвия, Рига" || res.Links[1].Name != "США, Канзас-Сити 2" {
		t.Fatalf("links: %+v", res.Links)
	}
	if !strings.HasPrefix(res.Links[0].URI, "awg2://192.0.2.10:41491?") {
		t.Fatalf("not converted: %s", res.Links[0].URI)
	}
	if len(res.Sources) != 1 {
		t.Fatalf("sources: %+v", res.Sources)
	}
	src := res.Sources[0]
	if src.Title != "PulseVPN 3" || src.SupportURL != "https://t.me/example_bot?start=support" || src.Expire != expire || src.Servers != 2 || src.Error != "" {
		t.Fatalf("source: %+v", src)
	}
	if strings.Contains(src.URL, "127.0.0.1:") && !strings.Contains(src.URL, "…") && strings.Count(src.URL, "/") > 2 {
		t.Fatalf("source URL is not masked: %s", src.URL)
	}
}

func TestFetchReportsAnExpiredSubscription(t *testing.T) {
	u := panelServer(t, nil, 0, http.StatusForbidden)
	res, err := NewFetcher().Fetch([]string{u})
	if err == nil {
		t.Fatal("a 403 must fail the fetch")
	}
	if !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "Подписка не активна") {
		t.Fatalf("error should carry the panel's reason: %v", err)
	}
	if len(res.Sources) != 1 || res.Sources[0].Error == "" {
		t.Fatalf("the failed source must be recorded: %+v", res.Sources)
	}
}

func TestEntryName(t *testing.T) {
	cases := map[string]string{
		"amneziawg://abc#%D0%A0%D0%B8%D0%B3%D0%B0": "Рига",
		"vless://u@h:443?x=1#My%20Server":          "My Server",
		"hy2://u@h:443":                            "",
		"ss://x#" + strings.Repeat("a", 60):        strings.Repeat("a", 48),
		"amneziawg://abc#  spaced   name ":         "spaced name",
	}
	for in, want := range cases {
		if got := entryName(in); got != want {
			t.Errorf("entryName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseExpire(t *testing.T) {
	if got := parseExpire("upload=0; download=0; total=0; expire=1823280907"); got != 1823280907 {
		t.Fatalf("got %d", got)
	}
	if got := parseExpire("upload=0"); got != 0 {
		t.Fatalf("no expire must be 0, got %d", got)
	}
	if got := parseExpire(""); got != 0 {
		t.Fatalf("empty header must be 0, got %d", got)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	old := paths.SubscriptionStatus
	paths.SubscriptionStatus = filepath.Join(t.TempDir(), "run", "subscriptions.json")
	t.Cleanup(func() { paths.SubscriptionStatus = old })

	if _, ok := ReadStatus(); ok {
		t.Fatal("no status yet")
	}
	st := Status{UpdatedAt: time.Now().Truncate(time.Second), OK: true, Sources: []Source{{URL: "https://p/…abcd", Title: "PulseVPN", Expire: 42, Servers: 5}}}
	if err := WriteStatus(st); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadStatus()
	if !ok || !got.OK || len(got.Sources) != 1 || got.Sources[0].Title != "PulseVPN" || got.Sources[0].Servers != 5 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestPlanNamesKeepsManualNamesAndReplacesOwned(t *testing.T) {
	links := []Link{
		{URI: "vless://uuid-a@1.1.1.1:443?x=1", Name: "Рига"},
		{URI: "vless://uuid-b@2.2.2.2:443?x=1", Name: "Амстердам"},
		{URI: "vless://uuid-c@3.3.3.3:443?x=1"},                 // unnamed: no entry
		{URI: "vless://uuid-a@1.1.1.1:443?x=2", Name: "Рига 2"}, // same server and account: one entry
	}
	idA, idB := channels.ID(links[0].URI), channels.ID(links[1].URI)
	if idA != channels.ID(links[3].URI) {
		t.Fatal("test setup: both Riga links must share an id")
	}
	existing := []string{"deadbeef=Мой ручной", idB + "=Старое имя от панели", idA + "=Моё имя для Риги"}
	ownedBefore := []string{idB + "=Старое имя от панели"}

	final, owned := planNames(existing, ownedBefore, links)
	wantFinal := []string{"deadbeef=Мой ручной", idA + "=Моё имя для Риги", idB + "=Амстердам"}
	if !reflect.DeepEqual(final, wantFinal) {
		t.Fatalf("final = %v, want %v", final, wantFinal)
	}
	if !reflect.DeepEqual(owned, []string{idB + "=Амстердам"}) {
		t.Fatalf("owned = %v", owned)
	}

	// Without a manual name the panel's name is written once per server.
	final, owned = planNames(nil, nil, links)
	if !reflect.DeepEqual(final, []string{idA + "=Рига", idB + "=Амстердам"}) || !reflect.DeepEqual(owned, final) {
		t.Fatalf("fresh: final=%v owned=%v", final, owned)
	}
}

func TestOwnedStateKeepsNamesAcrossLinkSaves(t *testing.T) {
	old := paths.SubscriptionState
	paths.SubscriptionState = filepath.Join(t.TempDir(), "links.json")
	t.Cleanup(func() { paths.SubscriptionState = old })

	saveOwnedState(ownedLinks{Section: "main", Links: []string{"a"}, Names: []string{"x=Y"}})
	saveOwned("main", []string{"a", "b"})
	st := loadOwnedState("main")
	if !reflect.DeepEqual(st.Links, []string{"a", "b"}) || !reflect.DeepEqual(st.Names, []string{"x=Y"}) {
		t.Fatalf("state = %+v", st)
	}
}
