package subscription

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// ClientIdentity is used for every subscription request, including redirects
// and tunnel retries. HWID stays stable across service and firmware updates.
type ClientIdentity struct {
	UserAgent   string
	HWID        string
	DeviceOS    string
	OSVersion   string
	DeviceModel string
}

func readIdentityFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func DefaultIdentity() ClientIdentity {
	model := readIdentityFile("/tmp/sysinfo/model")
	if model == "" {
		model = "OpenWrt Router"
	}
	mac := readIdentityFile("/sys/class/net/eth0/address")
	if mac == "" {
		mac = readIdentityFile("/sys/class/net/br-lan/address")
	}
	return ClientIdentity{HWID: routerHWID(mac, model), DeviceOS: "OpenWrt Linux", OSVersion: readIdentityFile("/proc/sys/kernel/osrelease"), DeviceModel: model}
}

// Match the stable OpenWrt identity used by NetShift on the same router so a
// migration need not consume another device slot. This hash is an identifier,
// not an authentication secret or a cryptographic integrity check.
func routerHWID(mac, model string) string {
	sum := md5.Sum([]byte(strings.TrimSpace(mac) + "-" + strings.TrimSpace(model)))
	h := hex.EncodeToString(sum[:8])
	return h[:4] + "-" + h[4:8] + "-" + h[8:12] + "-" + h[12:16]
}

func IdentityFromSettings(settings *uci.Section) (ClientIdentity, error) {
	id := DefaultIdentity()
	fields := []struct {
		key   string
		value *string
	}{
		{"subscription_user_agent", &id.UserAgent}, {"subscription_hwid", &id.HWID},
		{"subscription_device_os", &id.DeviceOS}, {"subscription_os_version", &id.OSVersion}, {"subscription_device_model", &id.DeviceModel},
	}
	for _, field := range fields {
		if raw := settings.Get(field.key, ""); strings.TrimSpace(raw) != "" {
			*field.value = strings.TrimSpace(raw)
		}
		if strings.ContainsAny(*field.value, "\r\n\x00") {
			return ClientIdentity{}, fmt.Errorf("%s contains invalid header characters", field.key)
		}
	}
	return id, nil
}

func (id ClientIdentity) userAgents() []string {
	if id.UserAgent != "" {
		return []string{id.UserAgent}
	}
	// URI clients come first: they avoid fetching a large full application config
	// when the same panel can supply the native engine's share links directly.
	return []string{"v2rayN", "Happ/1.0.0", "Hiddify", "singbox/1.12.0", "Clash.Meta"}
}

func (id ClientIdentity) headers(userAgent string) http.Header {
	h := make(http.Header)
	for key, value := range map[string]string{"User-Agent": userAgent, "X-HWID": id.HWID, "X-Device-OS": id.DeviceOS, "X-Ver-OS": id.OSVersion, "X-Device-Model": id.DeviceModel, "Accept-Language": "ru-RU,en,*", "X-Device-Locale": "EN"} {
		if value != "" {
			h.Set(key, value)
		}
	}
	return h
}
