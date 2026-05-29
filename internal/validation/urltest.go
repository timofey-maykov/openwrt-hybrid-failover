package validation

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateURLTestDurationPair checks sing-box rule: check_interval <= idle_timeout.
// Either value may be empty (skipped).
func ValidateURLTestDurationPair(checkInterval, idleTimeout string) error {
	checkInterval = strings.TrimSpace(checkInterval)
	idleTimeout = strings.TrimSpace(idleTimeout)
	if checkInterval == "" || idleTimeout == "" {
		return nil
	}
	intervalSec, err := SingboxDurationSeconds(checkInterval)
	if err != nil {
		return fmt.Errorf("urltest_check_interval: %w", err)
	}
	idleSec, err := SingboxDurationSeconds(idleTimeout)
	if err != nil {
		return fmt.Errorf("urltest_idle_timeout: %w", err)
	}
	if intervalSec > idleSec {
		return fmt.Errorf(
			"urltest_check_interval (%s) не может быть больше urltest_idle_timeout (%s): для sing-box нужно interval ≤ idle_timeout (например interval 30s, idle 5m)",
			checkInterval, idleTimeout,
		)
	}
	return nil
}

// SingboxDurationSeconds parses sing-box durations like 30s, 5m, 1h.
func SingboxDurationSeconds(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("пустая длительность")
	}
	if len(s) < 2 {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("некорректная длительность %q", s)
		}
		return n, nil
	}
	unit := s[len(s)-1]
	numStr := strings.TrimSpace(s[:len(s)-1])
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("некорректная длительность %q", s)
	}
	switch unit {
	case 's', 'S':
		return n, nil
	case 'm', 'M':
		return n * 60, nil
	case 'h', 'H':
		return n * 3600, nil
	case 'd', 'D':
		return n * 86400, nil
	default:
		return 0, fmt.Errorf("неизвестная единица в %q (используйте s, m, h)", s)
	}
}

func urltestPeerOption(option string) string {
	switch option {
	case "urltest_check_interval", "urltest_interval":
		return "urltest_idle_timeout"
	case "urltest_idle_timeout":
		return "urltest_check_interval"
	default:
		return ""
	}
}

func UCIOptionName(key string) string {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[i+1:]
	}
	return key
}

func UCIPeerKey(key, peerOption string) string {
	if peerOption == "" {
		return ""
	}
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[:i+1] + peerOption
	}
	return ""
}

// PeerURLTestUCIKey returns the paired urltest option key, or "" if not a urltest duration key.
func PeerURLTestUCIKey(key string) string {
	return UCIPeerKey(key, urltestPeerOption(UCIOptionName(key)))
}

// ValidateURLTestUCISet validates sing-box urltest timing after setting key to newValue.
// peerValue is the current UCI value of the paired option (may be empty).
func ValidateURLTestUCISet(key, newValue, peerValue string) error {
	opt := UCIOptionName(key)
	if urltestPeerOption(opt) == "" {
		return nil
	}
	var checkInterval, idleTimeout string
	switch opt {
	case "urltest_check_interval", "urltest_interval":
		checkInterval = newValue
		idleTimeout = peerValue
	case "urltest_idle_timeout":
		idleTimeout = newValue
		checkInterval = peerValue
	default:
		return nil
	}
	return ValidateURLTestDurationPair(checkInterval, idleTimeout)
}
