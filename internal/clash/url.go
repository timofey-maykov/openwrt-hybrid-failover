package clash

import (
	"fmt"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const defaultPort = 9090

// ResolveBaseURL returns the Clash API base URL (with http://) from UCI settings.
func ResolveBaseURL(settings *uci.Section) string {
	listen := ""
	if settings != nil {
		listen = settings.Get("clash_api_listen", "")
		if listen == "" {
			if addr := settings.Get("service_listen_address", ""); addr != "" {
				listen = formatListen(addr)
			}
		}
	}
	if listen == "" {
		listen = DetectListenAddress("")
	}
	if !strings.Contains(listen, "://") {
		listen = "http://" + listen
	}
	return listen
}

func formatListen(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DetectListenAddress("")
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, defaultPort)
}

// DetectListenAddress returns host:port for Clash API when settings are empty.
func DetectListenAddress(defaultLAN string) string {
	if defaultLAN != "" {
		return formatListen(defaultLAN)
	}
	if lan := detectLANAddress(); lan != "" {
		return fmt.Sprintf("%s:%d", lan, defaultPort)
	}
	return fmt.Sprintf("127.0.0.1:%d", defaultPort)
}

func detectLANAddress() string {
	out, err := uci.Exec("get", "network.lan.ipaddr")
	if err != nil {
		return ""
	}
	out = strings.TrimSpace(strings.Split(out, "/")[0])
	if out == "" {
		return ""
	}
	return out
}
