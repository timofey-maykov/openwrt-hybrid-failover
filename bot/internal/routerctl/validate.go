package routerctl

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// Every value that came from a chat message is checked here before it reaches
// a command. The commands are started without a shell (and shell-quoted over
// SSH), so this is a second line of defence and also keeps option-like words
// such as "-f" from being taken as arguments by ping or ifup.
var (
	reIface   = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
	reService = regexp.MustCompile(`^[A-Za-z0-9._@-]{1,64}$`)
	reHost    = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	reMAC     = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)
	reRadio   = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)
	reName    = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
)

// ValidHost accepts a host name or an IP address, never something that starts
// with a dash.
func ValidHost(h string) bool {
	h = strings.Trim(h, "[]")
	if net.ParseIP(h) != nil {
		return true
	}
	return reHost.MatchString(h)
}

func ValidIface(s string) bool   { return reIface.MatchString(s) }
func ValidService(s string) bool { return reService.MatchString(s) }
func ValidRadio(s string) bool   { return reRadio.MatchString(s) }
func ValidName(s string) bool    { return reName.MatchString(s) }

// NormalizeMAC returns the lower-case form, or false.
func NormalizeMAC(s string) (string, bool) {
	if !reMAC.MatchString(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

func ValidIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// NormalizePort accepts "80" or "8000-8010".
func NormalizePort(s string) (string, error) {
	parts := strings.Split(s, "-")
	if len(parts) > 2 {
		return "", fmt.Errorf("порт должен быть числом или диапазоном 8000-8010")
	}
	nums := make([]int, 0, 2)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("порт должен быть от 1 до 65535")
		}
		nums = append(nums, n)
	}
	if len(nums) == 2 && nums[0] >= nums[1] {
		return "", fmt.Errorf("в диапазоне портов первое число должно быть меньше второго")
	}
	if len(nums) == 1 {
		return strconv.Itoa(nums[0]), nil
	}
	return fmt.Sprintf("%d-%d", nums[0], nums[1]), nil
}
