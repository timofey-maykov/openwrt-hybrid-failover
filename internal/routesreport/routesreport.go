// Package routesreport describes the channels, service lists and per-list
// bindings of every vpn/proxy section: the ListRoutes RPC that the LuCI
// services tab and the bot read. Types only, so the bot does not link the engine.
package routesreport

// Report is the RPC answer.
type Report struct {
	OK       bool      `json:"ok"`
	Sections []Section `json:"sections"`
}

type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Up      *bool  `json:"up,omitempty"` // nil: no probe yet
	DelayMs int    `json:"delay_ms,omitempty"`
	Active  int64  `json:"active"`
	Rx      uint64 `json:"rx"`
	Tx      uint64 `json:"tx"`
	Weight  int    `json:"weight"` // sum of bound list weights
}

type List struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Kind    string `json:"kind"` // community, user, local, user_list
	Weight  int    `json:"weight"`
	Route   string `json:"route,omitempty"` // list_route that takes it
	Channel string `json:"channel"`         // channel id, auto, balance, direct, block
}

type Binding struct {
	Name    string   `json:"name"`
	Lists   []string `json:"lists"`
	Channel string   `json:"channel"`
	OnDown  string   `json:"on_down"`
	Current string   `json:"current,omitempty"`
	Via     string   `json:"via,omitempty"`
	Missing bool     `json:"missing,omitempty"`
}

type Section struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Channels []Channel         `json:"channels"`
	Lists    []List            `json:"lists"`
	Bindings []Binding         `json:"bindings"`
	Suggest  map[string]string `json:"suggest,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
}
