package security

import "strings"

type Authorizer struct {
	admin  map[int64]struct{}
	viewer map[int64]struct{}
}

func NewAuthorizer(adminIDs, viewerIDs []int64) Authorizer {
	out := Authorizer{
		admin:  make(map[int64]struct{}, len(adminIDs)),
		viewer: make(map[int64]struct{}, len(viewerIDs)),
	}
	for _, id := range adminIDs {
		out.admin[id] = struct{}{}
	}
	for _, id := range viewerIDs {
		out.viewer[id] = struct{}{}
	}
	return out
}

func (a Authorizer) IsAdmin(userID int64) bool {
	_, ok := a.admin[userID]
	return ok
}

func (a Authorizer) IsViewer(userID int64) bool {
	_, ok := a.viewer[userID]
	return ok
}

// Allowed reports whether the user may invoke cmd (viewers: read-only commands only).
func (a Authorizer) Allowed(userID int64, cmd string) bool {
	if a.IsAdmin(userID) {
		return true
	}
	if !a.IsViewer(userID) {
		return false
	}
	return isReadOnlyCommand(cmd)
}

func isReadOnlyCommand(cmd string) bool {
	cmd = strings.Fields(strings.TrimSpace(cmd))[0]
	switch cmd {
	case "/start", "/help", "/panel", "/quick", "/wizard", "/status", "/health",
		"/channels", "/history", "/failover_history", "/failover_list",
		"/uci_show", "/uci_sections", "/param_list", "/params", "/logs":
		return true
	default:
		return false
	}
}
