[English](en/CLI.md)

# Команды `hybrid-failover`

Бинарник core лежит в `/usr/sbin/hybrid-failover`. Те же действия делают LuCI и бот: они вызывают `hybrid-failover rpc <Method>`.

## Служба

| Команда | Что делает |
|---------|------------|
| `start` | Переключает dnsmasq на `127.0.0.42`, ставит правила nft, запускает engine, ставит cron списков и подписок |
| `stop` | Останавливает engine, снимает nft и cron, возвращает dnsmasq из backup |
| `restart` | `stop`, затем `start` |
| `reload` | Перечитывает UCI и обновляет engine |
| `monitor` | Процесс под procd: engine, контроллер failover и watchdog. Вручную не запускается |

Обычно службой управляют через `/etc/init.d/hybrid-failover start|stop|restart|enable`.

## Конфигурация

| Команда | Что делает |
|---------|------------|
| `migrate [--dry-run]` | Доводит UCI до текущей схемы, см. [OVERVIEW.md](OVERVIEW.md#миграция-схемы). Без изменений пишет `migration: no changes` |
| `validate [--dry-run] [--uci PATH]` | Проверяет URI, пары `urltest_check_interval` / `urltest_idle_timeout`, привязки `list_route` и собирает план engine |
| `apply [--dry-run] [--uci PATH]` | Применяет план engine и nft, при изменениях делает reload |
| `pending capture` | Дописывает `uci changes hybrid-failover` в снимок `/etc/hybrid-failover/pending` |
| `pending validate` | Проверяет снимок |
| `pending apply` | Коммитит staged-изменения (или проигрывает снимок) и применяет |
| `pending rollback` | Удаляет снимок |

`--uci PATH` проверяет или применяет другой файл вместо `/etc/config/hybrid-failover`. Как устроен pending, описано в [OVERVIEW.md](OVERVIEW.md#pending-конфиг-luci-и-бот).

## Списки и подписки

| Команда | Что делает |
|---------|------------|
| `list-update` | Скачивает community-списки в `/etc/hybrid-failover/rulesets/` и обновляет engine, если они изменились |
| `subscription-refresh [--dry-run] [--merge=false]` | Забирает ссылки из `settings.subscription_urls` и пишет их в `main_section` |

`subscription-refresh --dry-run` только скачивает и печатает число ссылок. По умолчанию ссылки, добавленные вручную, остаются, `--merge=false` заменяет список целиком. Флаг `--section` есть, но если задан `settings.main_section` (в шаблоне он задан), берётся `main_section`.

## Состояние и проверки

| Команда | Что делает |
|---------|------------|
| `status` | JSON: engine, nft, fakeip, активный outbound, контроллер |
| `health` | То же плюс живая проверка каналов |
| `global-check` | То же, что `status` |
| `check-nft` (или `check`) | Таблица `inet hybrid_failover` на месте |
| `check-fakeip` | DNS-запрос к `127.0.0.42`, ждёт адрес `198.18.x.x`. `dig` не нужен |
| `check-proxy [url]` | Обычный HTTP GET с роутера, по умолчанию на `https://www.gstatic.com/generate_204`. Проверяет выход в интернет, а не канал |

Поле `clash_ok` в отчёте осталось для совместимости и повторяет `engine_running`.

## Обновление

| Команда | Что делает |
|---------|------------|
| `update check` | Последний релиз на GitHub и есть ли обновление |
| `update apply [--tag vX.Y.Z] [--force] [--foreground]` | Ставит релиз. По умолчанию в фоне, `--foreground` в текущей консоли, `--force` разрешает ту же или более старую версию |
| `update status` | Ход и журнал последнего обновления |

Подробности в [INSTALL.md](INSTALL.md#релизы-и-обновление).

## RPC

`hybrid-failover rpc <Method> [json]` печатает JSON. Метод можно писать как `Status` или как `status`, для составных имён как `ListRoutes` или `list_routes`.

| Метод | Назначение |
|-------|------------|
| `Status`, `Health`, `GlobalCheck`, `CheckNft`, `CheckFakeip` | Состояние и проверки |
| `Validate`, `Apply` | Проверка и применение UCI |
| `CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` | Pending-конфиг |
| `History`, `ExportHistory`, `DelayHistory`, `Metrics` | Журнал переключений, история задержек, счётчики каналов |
| `SwitchProxy` | Ручное переключение selector секции |
| `DecodeURI` | Показать, как core разобрал ссылку |
| `ListClients` | Действующие правила клиентов |
| `ListRoutes` | Каналы секций и привязки списков |
| `ListUpdate`, `SubscriptionRefresh`, `SubscriptionStatus` | Списки и подписки |
| `DuplicateSection` | Копия секции маршрутизации |
| `BackupUCI`, `BackupDownload`, `RestoreUCI` | Резервная копия UCI |
| `UpdateCheck`, `UpdateApply`, `UpdateStatus` | Обновление |

Какие из них вызывает LuCI, перечислено в [luci/README.md](../luci/README.md).

## Журналы и файлы

```sh
logread -e hybrid-failover                     # системный лог core
cat /var/log/hybrid-failover/history.jsonl     # журнал переключений
cat /tmp/hybrid-failover/update/update.log     # журнал последнего обновления
```
