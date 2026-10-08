package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/audit"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/historywatch"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerctl"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/security"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/telegram"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/watchdog"
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
	logger := slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{ReplaceAttr: redactAttr}))
	mgr, err := routers.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("routers: %w", err)
	}
	store := botconfig.NewStore(configPath)
	handler := telegram.NewCommandHandler(mgr, store).WithShellIDs(cfg.AllowShellIDs)
	var wd *watchdog.Watchdog
	if cfg.WatchdogOn() {
		var targets []watchdog.Target
		for _, inst := range mgr.List() {
			targets = append(targets, watchdog.Target{ID: inst.ID, Name: inst.Name, Router: inst.Service})
		}
		wd = watchdog.New(targets, watchdog.Config{Interval: cfg.WatchdogInterval(), AutoRepair: cfg.WatchdogRepairs()}, logger)
		handler = handler.WithWatchdog(wd)
	}
	auth := security.NewAuthorizer(cfg.AdminIDs, cfg.ViewerIDs)
	auditLogger := audit.New(cfg.AuditPath)

	api, err := tgbotapi.NewBotAPIWithClient(cfg.Token, tgbotapi.APIEndpoint, telegram.NewHTTPClient(telegram.HTTPTimeout))
	if err != nil {
		return fmt.Errorf("create telegram client: %w", err)
	}
	identity := cfg.Identity()
	logger = logger.With("router", identity)
	if cfg.NotifyFailoverEnabled {
		interval := time.Duration(cfg.NotifyFailoverIntervalSeconds) * time.Second
		go historywatch.Run(ctx, api, cfg.AdminIDs, interval, identity)
	}
	bot := telegram.New(api, auth, auditLogger, handler, logger)
	bot.SetIdentity(identity, cfg.AdminIDs)
	if wd != nil {
		wd.SetNotifier(bot.NotifyAdmins)
		go wd.Run(ctx)
		logger.Info("watchdog started", "interval", cfg.WatchdogInterval().String(), "auto_repair", cfg.WatchdogRepairs())
	}
	for _, w := range mgr.Warnings {
		logger.Warn("config", "detail", w)
	}
	logger.Info("bot started", "routers", len(mgr.List()))
	return bot.Run(ctx)
}

// redactAttr hides secrets in log values. The Telegram client puts the whole
// request URL, bot token included, into its error text.
func redactAttr(_ []string, a slog.Attr) slog.Attr {
	switch v := a.Value.Any().(type) {
	case string:
		a.Value = slog.StringValue(routerctl.Redact(v))
	case error:
		a.Value = slog.StringValue(routerctl.Redact(v.Error()))
	}
	return a
}

// Exec runs one bot command without Telegram, as userID, for checking the
// command set from a shell on the router. It does not confirm anything: the
// caller is already root on the router.
func Exec(ctx context.Context, configPath string, userID int64, cmd string) (string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", err
	}
	if userID == 0 && len(cfg.AdminIDs) > 0 {
		userID = cfg.AdminIDs[0]
	}
	mgr, err := routers.NewManager(cfg)
	if err != nil {
		return "", fmt.Errorf("routers: %w", err)
	}
	h := telegram.NewCommandHandler(mgr, botconfig.NewStore(configPath)).WithShellIDs(cfg.AllowShellIDs)
	if nav, ok := strings.CutPrefix(cmd, "nav:"); ok {
		text, ok := h.Screen(ctx, userID, nav)
		if !ok {
			return "", fmt.Errorf("нет такого экрана: %s", nav)
		}
		return text, nil
	}
	if cmd == "/backup" {
		name, data, err := h.Backup(ctx, userID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %d bytes", name, len(data)), nil
	}
	return h.Handle(ctx, userID, cmd)
}
