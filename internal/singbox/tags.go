package singbox

import "strings"

func OutboundTag(section string) string {
	return section + "-out"
}

func URLTestTag(section string) string {
	return section + "-urltest-out"
}

func AWGTag(section string) string {
	return section + "-awg-out"
}

func PeerTag(section string, index int) string {
	return OutboundTag(section + "-" + itoa(index))
}

func RulesetTag(section, name, typ string) string {
	if typ != "" {
		return section + "-" + name + "-" + typ + "-ruleset"
	}
	return section + "-" + name + "-ruleset"
}

func DomainResolverTag(section string) string {
	return section + "-domain-resolver"
}

func NormalizeDuration(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, "smhdSMHD") {
		return v
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return v
		}
	}
	return v + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
