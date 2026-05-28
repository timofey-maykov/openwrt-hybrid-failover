# Podkop Telegram Bot (OpenWrt)

Легковесный Telegram-бот для управления Podkop и failover на роутере OpenWrt.

**Telegram:** свой бот (токен из [@BotFather](https://t.me/BotFather))  
**LuCI:** Сервисы → Telegram-бот Podkop  
**Установка:** [docs/INSTALL.md](../docs/INSTALL.md) или пакеты из [Releases](https://github.com/timofey-maykov/podkop-hybrid-failover/releases)

## Принципы

- Один бинарник (`/usr/bin/podkop-telegram-bot`)
- Минимум зависимостей (stdlib + Telegram SDK)
- Без дополнительных runtime (Python/Node)

## Сборка

```sh
cd bot
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags="-s -w" -o podkop-telegram-bot ./cmd/podkop-telegram-bot
```

## Конфиг

Файл: `/etc/podkop-telegram-bot.json`

Обязательные поля:
- `token`
- `admin_ids`

Поддерживаемые policy:
- `outage-only`
- `prefer-primary`

## Telegram команды

- `/status`
- `/quick` (быстрые сценарии)
- `/panel` (главная кнопочная панель)
- `/param_menu` (меню популярных операций)
- `/uci_menu` (полное UCI-управление podkop)
- `/params`
- `/param_list`
- `/param_get <key|alias>`
- `/param_set <key|alias> <value>`
- `/param_del <key|alias>`
- `/param_preview`
- `/param_apply`
- `/param_rollback`
- `/uci_show [podkop.section]`
- `/uci_sections`
- `/uci_get <key>`
- `/uci_set <key> <value>`
- `/uci_add_list <key> <value>`
- `/uci_del_list <key> <value>`
- `/uci_del <key>`
- `/set_quic on|off`
- `/set_policy outage-only|prefer-primary`
- `/set_urltest_interval <seconds>`

`/param_menu` теперь отправляет inline-кнопки:
- Статус
- Параметры
- QUIC ON/OFF
- Policy outage-only / prefer-primary
- Preview / Apply / Rollback

`/panel` открывает многоуровневую навигацию:
- Service
- Failover
- Params
- Config
- безопасные подтверждения для опасных действий (apply/rollback/restart)
- `/channels`
- `/health` (`/check_channels`): проверка доступности каналов
- `/podkop_restart`
- `/failover_list`
- `/failover_params`
- `/failover_help`
- `/failover_add <uri>`
- `/failover_rm <uri>`
- `/failover_apply`
- `/set_urltest_tolerance <ms>`
- `/set_urltest_idle_timeout <seconds>`
- `/set_interrupt_existing on|off`
- `/switch <outbound>`
- `/config_show`
- `/config_set <key> <value>`
- `/config_validate`
- `/config_apply`
- `/config_rollback`

## LuCI

Отдельная страница **Сервисы → Telegram-бот Podkop** (интерфейс на русском):

- включение и выключение сервиса;
- пути к бинарнику, конфигу и логам;
- редактирование JSON-конфига (токен, администраторы, Clash API, политика failover);
- действия: проверить / применить / откатить pending-конфиг, перезапустить бота.

## Деплой

**На роутере (рекомендуется):**

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/timofey-maykov/podkop-hybrid-failover/main/scripts/install-on-router.sh
PODKOP_HF_MODE=bot ash /tmp/install.sh
```

**Сборка пакетов на ПК:**

```sh
./scripts/build-packages.sh
# dist/ipk/podkop-telegram-bot_*_<arch>.ipk
```

**С хоста по SSH (разработка):**

```sh
./scripts/deploy-telegram-bot.sh 192.168.42.1
```
