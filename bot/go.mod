module github.com/tmaykov/openwrt-hybrid-failover/bot

go 1.25.0

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/tmaykov/openwrt-hybrid-failover v0.0.0
	golang.org/x/crypto v0.53.0
)

require golang.org/x/sys v0.46.0 // indirect

replace github.com/tmaykov/openwrt-hybrid-failover => ../
