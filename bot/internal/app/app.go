package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/audit"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerexec"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/security"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/telegram"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	logOut := io.Discard
	if cfg.LogPath != "" {
		f, ferr := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if ferr == nil {
			logOut = f
		}
	}
	logger := slog.New(slog.NewJSONHandler(logOut, nil))
	runner := routerexec.New(time.Duration(cfg.ProbeTimeoutSeconds) * time.Second)
	routingSvc := routing.NewService(runner, cfg.ClashAPI, cfg.RoutingInitScript, time.Duration(cfg.ProbeTimeoutSeconds)*time.Second)
	store := botconfig.NewStore(configPath)
	handler := telegram.NewCommandHandler(routingSvc, store)
	auth := security.NewAuthorizer(cfg.AdminIDs)
	auditLogger := audit.New(cfg.AuditPath)

	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return fmt.Errorf("create telegram client: %w", err)
	}
	bot := telegram.New(api, auth, auditLogger, handler, logger)
	return bot.Run(ctx)
}
