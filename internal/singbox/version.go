package singbox

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MinVersion is the minimum supported sing-box release.
const MinVersion = "1.12.4"

// CheckMinimumVersion verifies sing-box on PATH meets MinVersion.
func CheckMinimumVersion() error {
	out, err := exec.Command("sing-box", "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box version: %w: %s", err, strings.TrimSpace(string(out)))
	}
	ver := parseSingboxVersion(string(out))
	if ver == "" {
		return fmt.Errorf("sing-box version: could not parse output: %s", strings.TrimSpace(string(out)))
	}
	if compareVersion(ver, MinVersion) < 0 {
		return fmt.Errorf("sing-box %s is below required minimum %s", ver, MinVersion)
	}
	return nil
}

func parseSingboxVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "version" && i+1 < len(fields) {
				return strings.TrimPrefix(fields[i+1], "v")
			}
		}
		if strings.HasPrefix(line, "sing-box version") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return strings.TrimPrefix(parts[2], "v")
			}
		}
	}
	return ""
}

func compareVersion(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var va, vb int
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	base := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(base, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
