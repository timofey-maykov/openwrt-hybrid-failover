# Hybrid Failover Bot (OpenWrt)

> Не является Podkop. См. [docs/TRADEMARK.md](../docs/TRADEMARK.md).

Легковесный Telegram-бот для управления UCI **podkop** (upstream) и failover на роутере OpenWrt.

**Telegram:** свой бот (токен из [@BotFather](https://t.me/BotFather))  
**LuCI:** Сервисы → Telegram-бот Hybrid Failover  
**Установка:** [docs/INSTALL.md](../docs/INSTALL.md) или пакеты из [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases)

## Принципы

- Один бинарник (`/usr/bin/hybrid-failover-bot`)
- Минимум зависимостей (stdlib + Telegram SDK)
- Без дополнительных runtime (Python/Node)

## Сборка

```sh
cd bot
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags="-s -w" -o hybrid-failover-bot ./cmd/hybrid-failover-bot
```

## Конфиг

Файл: `/etc/hybrid-failover-bot.json`

Обязательные поля:
- `token`
- `admin_ids`

Опционально:
- `clash_api` (по умолчанию `http://127.0.0.1:9090`)
- `routing_init_script` (по умолчанию `/etc/init.d/podkop`; устаревший ключ `podkop_init_script` тоже читается)
- `policy`: `outage-only` | `prefer-primary`
- `probe_timeout_seconds`

UCI: `hybrid-failover-bot.main.enabled`, `binary`, `config_path`.

## Команды (основные)

- `/panel`, `/help`
- `/status`, `/routing_restart` (алиас `/podkop_restart`)
- `/health`, `/failover_list`, `/failover_add`, `/failover_apply`
- UCI: `/uci_get`, `/uci_set`, `/param_apply`, `/param_rollback`

Полный список: `/help` в боте.

## LuCI

Отдельная страница **Сервисы → Telegram-бот Hybrid Failover** (интерфейс на русском):

- редактирование JSON;
- pending-конфиг (apply/rollback);
- validate.

Путь: `/cgi-bin/luci/admin/services/hybrid-failover-bot`

## Установка одной командой

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
HF_MODE=bot ash /tmp/install.sh
```
