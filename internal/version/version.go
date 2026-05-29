package version

import (
	"os"
	"strings"
)

// Core is injected at link time: -ldflags "-X github.com/.../version.Core=1.5.0"
var Core = "dev"

func FromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return Core
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return Core
	}
	return v
}
