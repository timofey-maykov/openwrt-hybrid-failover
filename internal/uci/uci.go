package uci

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Package struct {
	Name     string
	Sections map[string]*Section
}

type Section struct {
	Type    string
	Name    string
	Options map[string]string
	Lists   map[string][]string
}

func Load(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

func Parse(text string) (*Package, error) {
	pkg := &Package{Sections: make(map[string]*Section)}
	var cur *Section

	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "config ") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				return nil, fmt.Errorf("invalid config line: %q", line)
			}
			secType := parts[1]
			secName := strings.Trim(parts[2], "'\"")
			cur = &Section{
				Type:    secType,
				Name:    secName,
				Options: make(map[string]string),
				Lists:   make(map[string][]string),
			}
			pkg.Sections[secName] = cur
			if pkg.Name == "" {
				pkg.Name = "hybrid-failover"
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "option ") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			key := parts[1]
			val := strings.Trim(strings.Join(parts[2:], " "), "'\"")
			cur.Options[key] = val
			continue
		}
		if strings.HasPrefix(line, "list ") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			key := parts[1]
			val := strings.Trim(strings.Join(parts[2:], " "), "'\"")
			cur.Lists[key] = append(cur.Lists[key], val)
		}
	}
	return pkg, sc.Err()
}

func (p *Package) Section(name string) *Section {
	if p == nil {
		return nil
	}
	return p.Sections[name]
}

func (s *Section) Get(key, def string) string {
	if s == nil {
		return def
	}
	if v, ok := s.Options[key]; ok {
		return v
	}
	return def
}

func (s *Section) GetBool(key string, def bool) bool {
	v := s.Get(key, "")
	if v == "" {
		return def
	}
	return v == "1" || strings.EqualFold(v, "true") || v == "yes"
}

func (s *Section) GetList(key string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.Lists[key]...)
}

func (p *Package) SectionNames(typ string) []string {
	var names []string
	for name, sec := range p.Sections {
		if sec.Type == typ {
			names = append(names, name)
		}
	}
	return names
}

func (p *Package) HasOutboundSection() bool {
	for _, sec := range p.Sections {
		if sec.Type != "section" {
			continue
		}
		if sec.Get("proxy_string", "") != "" ||
			sec.Get("interface", "") != "" ||
			sec.Get("outbound_json", "") != "" ||
			len(sec.GetList("urltest_proxy_links")) > 0 ||
			len(sec.GetList("failover_proxy_links")) > 0 {
			return true
		}
	}
	return false
}

// Exec runs uci on the router; for tests use Load/Parse instead.
func Exec(args ...string) (string, error) {
	cmd := exec.Command("uci", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("uci %v: %w: %s", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
