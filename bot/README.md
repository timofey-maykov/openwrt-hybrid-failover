# Hybrid Failover Bot (OpenWrt)

Легковесный Telegram-бот для управления UCI **`hybrid-failover`** и failover на роутере OpenWrt. Apply/restart/status выполняются через core RPC (`/usr/sbin/hybrid-failover`).

**Telegram:** свой бот (токен из [@BotFather](https://t.me/BotFather))  
**LuCI:** **Сервисы → Hybrid Failover → Telegram-бот**  
**Установка:** [docs/INSTALL.md](../docs/INSTALL.md) или пакеты из [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases)

## Принципы

- Один бинарник (`/usr/bin/hybrid-failover-bot`)
- Минимум зависимостей (stdlib + Telegram SDK)
- Без дополнительных runtime (Python/Node)
- Управление через core RPC

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
- `routing_init_script` (по умолчанию `/etc/init.d/hybrid-failover`)
- `policy`: `outage-only` | `prefer-primary`
- `probe_timeout_seconds`
- `viewer_ids`: только чтение (status/health/channels/history), без apply/uci_set
- `notify_failover_enabled` (default `false`): push в Telegram при новой строке в `history.jsonl`
- `notify_failover_interval_seconds` (default `30`): интервал poll

Алерты без бота: UCI `hybrid-failover.settings.webhook_url`: core шлёт HTTP POST при switch; см. [docs/OVERVIEW.md](../docs/OVERVIEW.md#алерты-при-failover).

UCI: `hybrid-failover-bot.main.enabled`, `binary`, `config_path`.

## Команды (основные)

- `/panel`, `/help`
- `/status`, `/routing_restart`
- `/health`, `/channels`, `/history`, `/failover_list`, `/failover_add`, `/failover_apply`
- UCI: `/uci_get`, `/uci_set`, `/param_apply`, `/param_rollback`
- Параметры: `/param_get disable_quic`, `/param_set cache_path …`

Полный список: `/help` в боте.

## LuCI

**Сервисы → Hybrid Failover → Telegram-бот** (интерфейс на русском):

- редактирование JSON;
- pending-конфиг (apply/rollback);
- validate.

Путь: `/cgi-bin/luci/admin/services/hybrid-failover/bot`

## Установка одной командой

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
HF_MODE=bot ash /tmp/install.sh
```

Требуется установленный **`hybrid-failover-core`** (в режиме `full` ставится автоматически).
