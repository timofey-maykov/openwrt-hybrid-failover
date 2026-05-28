package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/app"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
)

func main() {
	configPath := flag.String("config", "/etc/hybrid-failover-bot.json", "Path to bot config")
	mode := flag.String("mode", "run", "run|validate-config|apply-config|rollback-config|set-pending")
	key := flag.String("key", "", "pending key for set-pending")
	value := flag.String("value", "", "pending value for set-pending")
	flag.Parse()

	switch *mode {
	case "run":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := app.Run(ctx, *configPath); err != nil && err != context.Canceled {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "validate-config":
		store := botconfig.NewStore(*configPath)
		exitIfErr(store.ValidatePending())
	case "apply-config":
		store := botconfig.NewStore(*configPath)
		exitIfErr(store.ApplyPending())
	case "rollback-config":
		store := botconfig.NewStore(*configPath)
		exitIfErr(store.RollbackPending())
	case "set-pending":
		if *key == "" {
			exitIfErr(fmt.Errorf("-key is required"))
		}
		store := botconfig.NewStore(*configPath)
		exitIfErr(store.SetPendingKey(*key, *value))
	default:
		exitIfErr(fmt.Errorf("unknown mode %q", *mode))
	}
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
