package routerctl

import "regexp"

var (
	reBotToken = regexp.MustCompile(`(?:bot)?\d{6,}:[A-Za-z0-9_-]{25,}`)
	// uci export style: option password 'x'
	reUCISecret = regexp.MustCompile(`(?i)\b(option|list)(\s+)(password|passwd|psk|key|private_?key|secret|token|auth_?key|wpa_?key)(\s+)['"]?[^\s'"]+['"]?`)
	reSecretKV  = regexp.MustCompile(`(?i)\b(password|passwd|psk|key|private_?key|secret|token|auth_?key|wpa_?key)\b(['"]?\s*[=:]\s*['"]?)[^\s'",;]+`)
)

// Redact hides secrets in text that is about to be sent to a chat: Telegram
// bot tokens and values of password-like options.
func Redact(s string) string {
	s = reBotToken.ReplaceAllString(s, "<token>")
	s = reUCISecret.ReplaceAllString(s, "${1}${2}${3}${4}<скрыто>")
	return reSecretKV.ReplaceAllString(s, "${1}${2}<скрыто>")
}
