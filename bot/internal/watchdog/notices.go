package watchdog

// Button is a key under a notice: a bot command (Data, as callback data) or a
// link (URL). Exactly one of the two is set.
type Button struct {
	Text string
	Data string
	URL  string
}

// SetRichNotifier sets where notices with buttons go. Without it they fall
// back to the plain notifier, buttons dropped.
func (w *Watchdog) SetRichNotifier(send func(text string, rows [][]Button)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sendRich = send
}

// notifyRich sends text with buttons unless muted. Caller holds no lock.
func (w *Watchdog) notifyRich(text string, rows [][]Button) {
	w.mu.Lock()
	rich, plain, muted := w.sendRich, w.send, w.now().Before(w.mutedUntil)
	w.mu.Unlock()
	if muted {
		w.log.Info("watchdog notice withheld", "muted", true, "text", oneLine(text, 120))
		return
	}
	switch {
	case rich != nil:
		rich(text, rows)
	case plain != nil:
		plain(text)
	}
}
