package routerctl

import (
	"context"
	"fmt"
	"strings"
)

// ------------------------------------------------------------- packages

func (s Service) APKCheck(ctx context.Context) (string, error) {
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	if _, err := s.r.Run(cctx, "apk", "update"); err != nil {
		return "", fmt.Errorf("apk update: %v", shortErr(err))
	}
	out, err := s.r.Run(cctx, "apk", "list", "--upgradable")
	if err != nil {
		return "", fmt.Errorf("apk list: %v", shortErr(err))
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if f := strings.Fields(l); len(f) > 0 {
				lines = append(lines, f[0])
			}
		}
	}
	if len(lines) == 0 {
		return "Все пакеты актуальны.", nil
	}
	head := fmt.Sprintf("Можно обновить пакетов: %d", len(lines))
	if len(lines) > 40 {
		lines = append(lines[:40], fmt.Sprintf("… и ещё %d", len(lines)-40))
	}
	return head + "\n" + strings.Join(lines, "\n") + "\n\nОбновить все: /apk_upgrade", nil
}

func (s Service) APKUpgrade(ctx context.Context) (string, error) {
	cctx, cancel := withBudget(ctx, longTimeout)
	defer cancel()
	if _, err := s.r.Run(cctx, "apk", "update"); err != nil {
		return "", fmt.Errorf("apk update: %v", shortErr(err))
	}
	out, err := s.r.Run(cctx, "apk", "upgrade")
	if err != nil {
		return "", fmt.Errorf("apk upgrade: %v", shortErr(err))
	}
	return "Обновление пакетов завершено.\n" + clip(Redact(tailLines(out, 25)), 3000), nil
}

// ---------------------------------------------------------------- backup

const backupFile = "/tmp/hf-bot-backup.tar.gz"

// Backup returns the configuration archive (sysupgrade -b).
func (s Service) Backup(ctx context.Context) ([]byte, error) {
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	defer func() { _, _ = s.r.Run(context.Background(), "rm", "-f", backupFile) }()
	if _, err := s.r.Run(cctx, "sysupgrade", "-b", backupFile); err != nil {
		return nil, fmt.Errorf("sysupgrade -b: %v", shortErr(err))
	}
	data, err := s.r.RunBytes(cctx, "cat", backupFile)
	if err != nil {
		return nil, err
	}
	if len(data) < 100 || data[0] != 0x1f || data[1] != 0x8b {
		return nil, fmt.Errorf("архив получился неверным")
	}
	return data, nil
}

// ------------------------------------------------------- Beam WRT extras

const (
	beamUpdate = "/usr/sbin/be7000-update"
	beamSlots  = "/usr/libexec/be7000-slots"
	beamSplit  = "/usr/sbin/be7000-5g-split"
)

func (s Service) HasBeam(ctx context.Context) (update, slots, split bool) {
	return s.have(ctx, beamUpdate), s.have(ctx, beamSlots), s.have(ctx, beamSplit)
}

func (s Service) BeamUpdateCheck(ctx context.Context) (string, error) {
	if !s.have(ctx, beamUpdate) {
		return "", fmt.Errorf("на этом роутере нет be7000-update (это не Beam WRT)")
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, beamUpdate, "check")
	if err != nil {
		return "", fmt.Errorf("%v", shortErr(err))
	}
	return out, nil
}

func (s Service) BeamUpdateApply(ctx context.Context) (string, error) {
	if !s.have(ctx, beamUpdate) {
		return "", fmt.Errorf("на этом роутере нет be7000-update (это не Beam WRT)")
	}
	cctx, cancel := withBudget(ctx, longTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, beamUpdate, "apply", "--yes")
	if err != nil && !connectionDropped(err) {
		return "", fmt.Errorf("%v", shortErr(err))
	}
	if out == "" {
		out = "Обновление запущено, роутер перезагрузится."
	}
	return clip(out, 2000), nil
}

func (s Service) Slots(ctx context.Context) (string, error) {
	if !s.have(ctx, beamSlots) {
		return "", fmt.Errorf("на этом роутере нет be7000-slots (это не Beam WRT)")
	}
	out, err := s.r.Run(ctx, beamSlots, "status")
	if err != nil {
		return "", err
	}
	return "Слоты прошивки (JSON):\n" + out, nil
}

var splitModes = map[string]bool{"single": true, "split": true, "mlo": true}

func (s Service) Mode5G(ctx context.Context, mode string) (string, error) {
	if !s.have(ctx, beamSplit) {
		return "", fmt.Errorf("на этом роутере нет be7000-5g-split (это не Beam WRT)")
	}
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	if mode == "" {
		out, err := s.r.Run(cctx, beamSplit, "mode")
		if err != nil {
			return "", err
		}
		return "Режим 5 ГГц: " + out + "\nСменить: /mode5g single|split|mlo", nil
	}
	if !splitModes[mode] {
		return "", fmt.Errorf("режим: single, split или mlo")
	}
	out, err := s.r.Run(cctx, beamSplit, "mode", mode)
	if err != nil {
		return "", err
	}
	return "Режим 5 ГГц: " + mode + "\n" + clip(Redact(out), 1500), nil
}

// ----------------------------------------------------------------- shell

// Shell runs a command line through sh. It is only reachable for the Telegram
// users listed in allow_shell_ids, and always behind a confirmation.
func (s Service) Shell(ctx context.Context, cmdline string) (string, error) {
	cctx, cancel := withBudget(ctx, midTimeout)
	defer cancel()
	out, err := s.r.Run(cctx, "sh", "-c", cmdline)
	if err != nil {
		return "", fmt.Errorf("%v", clip(Redact(err.Error()), 3000))
	}
	if out == "" {
		return "(команда выполнена, вывода нет)", nil
	}
	return clip(Redact(out), 3500), nil
}
