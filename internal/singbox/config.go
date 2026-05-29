package singbox

import (
	"encoding/json"
)

type Config struct {
	Log          map[string]any   `json:"log"`
	DNS          map[string]any   `json:"dns"`
	NTP          map[string]any   `json:"ntp"`
	Certificate  map[string]any   `json:"certificate"`
	Endpoints    []any            `json:"endpoints"`
	Inbounds     []map[string]any `json:"inbounds"`
	Outbounds    []map[string]any `json:"outbounds"`
	Route        map[string]any   `json:"route"`
	RuleSets     []map[string]any `json:"-"`
	Services     []any            `json:"services"`
	Experimental map[string]any   `json:"experimental"`
}

func NewConfig() *Config {
	return &Config{
		Log:          map[string]any{},
		DNS:          map[string]any{},
		NTP:          map[string]any{},
		Certificate:  map[string]any{},
		Endpoints:    []any{},
		Inbounds:     []map[string]any{},
		Outbounds:    []map[string]any{},
		Route:        map[string]any{},
		Services:     []any{},
		Experimental: map[string]any{},
	}
}

func (c *Config) AddOutbound(ob map[string]any) {
	c.Outbounds = append(c.Outbounds, ob)
}

func (c *Config) AddInbound(in map[string]any) {
	c.Inbounds = append(c.Inbounds, in)
}

func (c *Config) AddRuleSet(rs map[string]any) {
	c.RuleSets = append(c.RuleSets, rs)
}

func (c *Config) JSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c *Config) MustJSON() []byte {
	b, err := c.JSON()
	if err != nil {
		panic(err)
	}
	return b
}
