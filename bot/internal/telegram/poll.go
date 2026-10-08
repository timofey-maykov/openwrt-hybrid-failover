package telegram

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Telegram rejects messages over 4096 UTF-16 units; stay below it with room
// for the router prefix and characters outside the BMP.
const maxMessageRunes = 3500

// pollTimeoutSeconds is the getUpdates long-poll length. The HTTP client must
// give up well after it, so a connection that died silently is dropped within
// seconds instead of leaving the bot deaf.
const (
	pollTimeoutSeconds = 30
	HTTPTimeout        = 45 * time.Second
)

// NewHTTPClient is the client for the Telegram API. A router reloads its
// firewall and routing while the bot is running (apply, failover), which can
// leave an open connection dead without any error; without a deadline the
// long poll would wait on it forever and no button would ever answer again.
// HTTP/1.1 without keep-alive is used on purpose: nothing stale is ever reused,
// and a timed-out request closes its own connection.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			// A router reload leaves an idle kept-alive connection dead without
			// telling anyone, and the next request on it hangs until the
			// timeout. A fresh connection per request costs one handshake.
			DisableKeepAlives: true,
			TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
}

// conflictNotifyEvery limits how often admins hear about a shared token.
const conflictNotifyEvery = time.Hour

// poll long-polls getUpdates. Unlike GetUpdatesChan it surfaces errors, so a
// token shared by two bot instances (HTTP 409 Conflict) is reported instead of
// updates silently landing on whichever router polled first.
func (b *Bot) poll(ctx context.Context, handle func(context.Context, tgbotapi.Update)) error {
	offset := 0
	failures := 0
	var lastConflictNotice time.Time
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		cfg := tgbotapi.NewUpdate(offset)
		cfg.Timeout = pollTimeoutSeconds
		updates, err := b.getUpdates(ctx, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failures++
			wait := backoff(failures)
			if isConflict(err) {
				wait = 30 * time.Second
				b.log.Error("telegram token is polled by another bot instance", "router", b.identity, "err", err)
				if time.Since(lastConflictNotice) >= conflictNotifyEvery {
					lastConflictNotice = time.Now()
					b.notifyAdmins(conflictNotice(b.identity))
				}
			} else {
				b.log.Error("telegram getUpdates failed", "err", err, "retry_in", wait.String())
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		failures = 0
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			handle(ctx, u)
		}
	}
}

// getUpdates returns early on shutdown instead of waiting out the long poll.
func (b *Bot) getUpdates(ctx context.Context, cfg tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	type result struct {
		updates []tgbotapi.Update
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		u, err := b.api.GetUpdates(cfg)
		ch <- result{u, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.updates, r.err
	}
}

func isConflict(err error) bool {
	var tgErr *tgbotapi.Error
	if errors.As(err, &tgErr) {
		return tgErr.Code == 409
	}
	return strings.Contains(err.Error(), "Conflict")
}

func backoff(failures int) time.Duration {
	d := 3 * time.Second
	for i := 1; i < failures && d < time.Minute; i++ {
		d *= 2
	}
	if d > time.Minute {
		d = time.Minute
	}
	return d
}

func conflictNotice(identity string) string {
	who := identity
	if who == "" {
		who = "этом роутере"
	}
	return fmt.Sprintf("⚠ Бот на %s не получает команды: этот токен опрашивает другой экземпляр бота.\n\n"+
		"Telegram отдаёт обновления только одному получателю, поэтому команды уходят на случайный роутер.\n"+
		"Варианты:\n"+
		"1) у каждого роутера свой бот: новый токен в @BotFather и в token файла /etc/hybrid-failover-bot.json;\n"+
		"2) один бот на главном роутере, остальные роутеры в его списке routers (по SSH), а на них бот выключить.", who)
}

func (b *Bot) notifyAdmins(text string) {
	for _, id := range b.adminIDs {
		b.sendChunks(id, text, nil)
	}
}

// splitMessage cuts text into parts that fit a Telegram message, preferring
// line boundaries.
func splitMessage(text string) []string {
	if utf8.RuneCountInString(text) <= maxMessageRunes {
		return []string{text}
	}
	var parts []string
	var cur strings.Builder
	curRunes := 0
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
			curRunes = 0
		}
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		n := utf8.RuneCountInString(line)
		if curRunes+n > maxMessageRunes {
			flush()
		}
		for n > maxMessageRunes {
			r := []rune(line)
			parts = append(parts, string(r[:maxMessageRunes]))
			line = string(r[maxMessageRunes:])
			n = len(r) - maxMessageRunes
		}
		cur.WriteString(line)
		curRunes += n
	}
	flush()
	return parts
}
