package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
)

// Subscriptions is the list of subscription URLs and how often core refetches
// them. Values include changes that are not applied yet.
type Subscriptions struct {
	URLs     []string
	Interval string
}

var subIntervalRe = regexp.MustCompile(`^[1-9][0-9]{0,3}[mhd]$`)

const maxSubscriptionURL = 2048

// ValidateSubscriptionURL accepts only plain http(s) links without anything a
// shell or UCI would trip over.
func ValidateSubscriptionURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("пустая ссылка")
	}
	if len(raw) > maxSubscriptionURL {
		return fmt.Errorf("ссылка слишком длинная")
	}
	if strings.ContainsAny(raw, " \t\r\n'\"`\\") {
		return fmt.Errorf("в ссылке есть пробелы или кавычки")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("нужна ссылка вида https://host/путь")
	}
	return nil
}

// ValidateSubscriptionInterval accepts 30m, 6h, 1d and the like.
func ValidateSubscriptionInterval(v string) error {
	if !subIntervalRe.MatchString(strings.TrimSpace(v)) {
		return fmt.Errorf("интервал вида 30m, 6h или 1d")
	}
	return nil
}

func (s Service) Subscriptions(ctx context.Context) (Subscriptions, error) {
	var out Subscriptions
	// uci exits non-zero for an option that is not set; that is an empty list.
	if raw, err := s.runner.Run(ctx, "/sbin/uci", "-q", "get", s.settingsKey("subscription_urls")); err == nil {
		out.URLs = strings.Fields(raw)
	}
	if raw, err := s.runner.Run(ctx, "/sbin/uci", "-q", "get", s.settingsKey("subscription_update_interval")); err == nil {
		out.Interval = strings.TrimSpace(raw)
	}
	return out, nil
}

func (s Service) AddSubscription(ctx context.Context, raw string) error {
	raw = strings.TrimSpace(raw)
	if err := ValidateSubscriptionURL(raw); err != nil {
		return err
	}
	cur, err := s.Subscriptions(ctx)
	if err != nil {
		return err
	}
	for _, u := range cur.URLs {
		if u == raw {
			return fmt.Errorf("такая подписка уже есть")
		}
	}
	if _, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci add_list %s=%s", s.settingsKey("subscription_urls"), shellQuote(raw))); err != nil {
		return err
	}
	return s.capturePending(ctx)
}

// SubscriptionAt returns the URL at the 1-based position n.
func (s Service) SubscriptionAt(ctx context.Context, n int) (string, error) {
	cur, err := s.Subscriptions(ctx)
	if err != nil {
		return "", err
	}
	if n < 1 || n > len(cur.URLs) {
		return "", fmt.Errorf("подписки №%d нет, список изменился", n)
	}
	return cur.URLs[n-1], nil
}

func (s Service) DelSubscription(ctx context.Context, n int) (string, error) {
	u, err := s.SubscriptionAt(ctx, n)
	if err != nil {
		return "", err
	}
	if _, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci del_list %s=%s", s.settingsKey("subscription_urls"), shellQuote(u))); err != nil {
		return "", err
	}
	return u, s.capturePending(ctx)
}

func (s Service) SetSubscriptionInterval(ctx context.Context, v string) error {
	v = strings.TrimSpace(v)
	if err := ValidateSubscriptionInterval(v); err != nil {
		return err
	}
	if _, err := s.runner.Run(ctx, "/bin/sh", "-lc", fmt.Sprintf("uci set %s=%s", s.settingsKey("subscription_update_interval"), shellQuote(v))); err != nil {
		return err
	}
	return s.capturePending(ctx)
}

// PendingCount is the number of staged UCI changes waiting for apply.
func (s Service) PendingCount(ctx context.Context) int {
	out, err := s.runner.Run(ctx, "/sbin/uci", "changes", s.uciPackage)
	if err != nil {
		return 0
	}
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

// StatusReport is the structured core status (what Status formats as text).
func (s Service) StatusReport(ctx context.Context) (diag.Report, error) {
	out, err := s.runner.RunCoreRPC(ctx, "Status")
	if err != nil {
		return diag.Report{}, err
	}
	var r diag.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return diag.Report{}, fmt.Errorf("status: %w", err)
	}
	return r, nil
}

// HealthReport is the structured result of fresh channel probes.
func (s Service) HealthReport(ctx context.Context) (diag.Report, error) {
	ctx, cancel := withTimeout(ctx, healthTimeout)
	defer cancel()
	out, err := s.runner.RunCoreRPC(ctx, "Health")
	if err != nil {
		return diag.Report{}, err
	}
	var r diag.Report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return diag.Report{}, fmt.Errorf("health: %w", err)
	}
	return r, nil
}
