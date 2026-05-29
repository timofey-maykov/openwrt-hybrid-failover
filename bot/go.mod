module github.com/tmaykov/openwrt-hybrid-failover/bot

go 1.22

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/tmaykov/openwrt-hybrid-failover v0.0.0
)

replace github.com/tmaykov/openwrt-hybrid-failover => ../
