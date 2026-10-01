[English](README.en.md)

# Hybrid Failover Bot

Telegram-бот для управления Hybrid Failover на роутере OpenWrt. Он показывает статус и каналы, меняет UCI `hybrid-failover` через pending, применяет изменения и присылает уведомления о переключениях. Статус, проверка каналов, история и apply идут через core RPC (`hybrid-failover rpc`), так что бот работает одинаково на локальном роутере и на удалённом по SSH.

Для каждой установки создаётся свой бот, токен выдаёт [@BotFather](https://t.me/BotFather). В LuCI настройки находятся в **Сервисы → Hybrid Failover → Telegram**. Установка описана в [docs/INSTALL.md](../docs/INSTALL.md), пакеты лежат в [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases).

Бот состоит из одного бинарника `/usr/bin/hybrid-failover-bot` без Python и Node. Кроме стандартной библиотеки Go он использует telegram-bot-api и `x/crypto/ssh`.

## Сборка

Пакеты для всех архитектур собирает `./scripts/build-packages.sh` (см. [packages/README.md](../packages/README.md)). Вручную бинарник собирается так, пример для `mipsel_24kc`:

```sh
cd bot
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
  go build -trimpath -ldflags="-s -w" -o hybrid-failover-bot ./cmd/hybrid-failover-bot
```

Модуль бота подключает core через `replace ../`, поэтому собирать нужно из полного репозитория.

## Конфиг

Файл `/etc/hybrid-failover-bot.json`. Пакет ставит его в варианте для одного роутера, без списка `routers`.

Обязательные ключи `token` и `admin_ids`. Токен можно передать и через переменную окружения `HF_BOT_TOKEN`, она важнее значения в файле.

| Ключ | По умолчанию | Смысл |
|------|--------------|-------|
| `router_name` | hostname | имя роутера в `/panel`, `/status` и уведомлениях о failover |
| `viewer_ids` | `[]` | пользователи только для чтения, см. ниже |
| `notify_failover_enabled` | `false` | присылать администраторам новые события failover |
| `notify_failover_interval_seconds` | `30` | как часто проверять историю, не меньше 10 |
| `log_path` | `/var/log/hybrid-failover-bot.log` | лог бота |
| `audit_path` | `/var/log/hybrid-failover-bot.audit.log` | журнал команд с ID пользователя и результатом |
| `routing_init_script` | `/etc/init.d/hybrid-failover` | его вызывает `/routing_restart` |
| `uci_package` | `hybrid-failover` | пакет UCI |
| `main_section` | `glob` | секция, с которой работают `/set_*`, `/failover_*` и алиасы |
| `probe_timeout_seconds` | `5` (в шаблоне 15) | таймаут HTTP-проверок через Clash API |
| `clash_api` | `http://127.0.0.1:9090` | запасной путь для старых установок на внешнем sing-box. Native engine Clash API не поднимает, и бот берёт каналы из core RPC |
| `policy` | `outage-only` | проверяется и показывается в `/config_show`, на маршрутизацию не влияет. Политику failover меняет `/set_policy` |
| `routers` | нет | список роутеров для схемы с одним ботом на несколько роутеров |

Если `main_section` нет в UCI локального роутера, бот берёт `hybrid-failover.settings.main_section` (по умолчанию `glob`) и пишет об этом в лог.

Командой `/config_set` и со страницы LuCI меняются ключи `token`, `router_name`, `admin_ids`, `viewer_ids`, `policy`, `clash_api`, `routing_init_script`, `log_path`, `audit_path`, `probe_timeout_seconds`, `notify_failover_enabled`, `notify_failover_interval_seconds`. Списки ID пишутся через запятую или пробел. Изменения сначала попадают в `/etc/hybrid-failover-bot.json.pending`. `/config_apply` проверяет pending и записывает его в основной файл. Бот читает конфиг только при старте, поэтому после этого его нужно перезапустить:

```sh
/etc/init.d/hybrid-failover-bot restart
```

`routers`, `uci_package` и `main_section` правятся только в самом JSON.

UCI `hybrid-failover-bot.main` содержит `enabled` (после установки `0`), `binary`, `config_path` и `log_path`.

Уведомления о failover без бота умеет присылать сам core через UCI `hybrid-failover.settings.webhook_url`, см. [docs/OVERVIEW.md](../docs/OVERVIEW.md#алерты-при-failover).

### Уведомления о failover

При `notify_failover_enabled: true` бот читает `/var/log/hybrid-failover/history.jsonl` и отправляет новые события всем `admin_ids`. Читатели из `viewer_ids` уведомлений не получают. В заголовке сообщения стоит имя роутера. Позиция хранится по времени последнего отправленного события, поэтому ротация файла core не приводит к пропускам или повторной отправке всей истории.

### Права доступа

Администраторы из `admin_ids` могут всё. Пользователи из `viewer_ids` могут вызывать только `/start`, `/help`, `/panel`, `/quick`, `/wizard`, `/status`, `/health`, `/channels`, `/routes`, `/history`, `/failover_history`, `/failover_list`, `/uci_show`, `/uci_sections`, `/params`, `/param_list`, `/logs`, `/check_channels`, `/clients`, а также выбор роутера через `/routers`, `/use` и `/router`. Выбор влияет только на то, что видит сам пользователь. В панели они переходят по разделам и нажимают кнопки этих же команд. Кнопки с вводом значения и подтверждением им недоступны. Остальным пользователям бот отвечает отказом. Каждая команда пишется в audit-лог.

## Несколько роутеров

Telegram отдаёт обновления одного токена только одному получателю. Если один токен прописан в ботах на двух роутерах, они по очереди перехватывают команды, и команда выполняется на случайном роутере. Рабочих схем две.

### У каждого роутера свой бот

Подходит, когда роутеры независимы.

1. В [@BotFather](https://t.me/BotFather) создайте по боту на роутер, например `home_hf_bot` и `office_hf_bot`.
2. На каждом роутере поставьте пакет бота и пропишите в `/etc/hybrid-failover-bot.json` его собственный `token`.
3. Список `routers` не задавайте. Бот управляет тем роутером, на котором запущен.
4. Задайте `router_name` или оставьте пустым, тогда возьмётся hostname. Имя видно в `/panel`, `/status` и в уведомлениях, так что в списке чатов сразу понятно, какой роутер пишет.

Бот сам замечает общий токен. Если getUpdates возвращает HTTP 409, бот раз в час пишет администраторам предупреждение, а в лог уходит `telegram token is polled by another bot instance`.

### Один бот на все роутеры по SSH

Подходит, когда нужен один чат на все роутеры.

Бот ставится на один хост с сетевым доступом к остальным роутерам. На остальных бот выключен (`uci set hybrid-failover-bot.main.enabled=0`), там нужен только core. Полный пример конфига лежит в [examples/hybrid-failover-bot.multi-router.json](../examples/hybrid-failover-bot.multi-router.json).

```json
"routers": [
  { "id": "home", "name": "Дом", "local": true },
  {
    "id": "office",
    "name": "Офис",
    "host": "192.168.11.1",
    "user": "root",
    "identity_file": "/etc/hybrid-failover-bot/id_office"
  }
]
```

- `local: true` обозначает роутер, где запущен бот. Команды идут в локальный `hybrid-failover`.
- Для удалённого роутера обязательны `host` и `identity_file`. `port` по умолчанию 22, `user` по умолчанию `root`. Бот выполняет те же команды по SSH с этим ключом. Публичный ключ должен лежать в `/etc/dropbear/authorized_keys` на роутере. Ключ хоста не проверяется.
- У каждого роутера можно отдельно задать `uci_package`, `main_section`, `routing_init_script` и `clash_api`. Без них берутся значения верхнего уровня.
- Роутер без файла ключа на диске пропускается при старте, в лог пишется предупреждение.
- `/routers` показывает список, `/use office` выбирает роутер, `/router` показывает текущий. Выбор свой у каждого администратора и хранится в памяти, после перезапуска бота его нужно сделать заново. Пока роутер не выбран, команды для роутера не выполняются.
- Ответы начинаются с `[Офис]`, в `/panel` появляются кнопки выбора роутера.
- Статус, каналы и история берутся через RPC core на выбранном роутере, поэтому работают и по SSH.
- Уведомления о failover приходят только с роутера, где запущен бот.

## Команды

Полный список с учётом текущей секции выводит `/help`.

**Общие**

- `/start`, `/help` выводят список команд.
- `/panel` открывает панель с кнопками.
- `/quick` (или `/wizard`) показывает типовые сценарии.
- `/param_menu` показывает подсказки по параметрам и кнопки быстрых настроек, `/uci_menu` подсказки по командам UCI.
- `/cancel` отменяет ожидаемый ввод значения.

**Роутеры** (когда их больше одного)

- `/routers` показывает список.
- `/use <id>` выбирает роутер.
- `/router` показывает выбранный.

**Состояние**

- `/status` показывает состояние службы и активные каналы.
- `/health` (или `/check_channels`) проверяет каналы заново через RPC Health.
- `/channels` (или `/failover_list`) показывает каналы и их доступность.
- `/routes` показывает каналы секций с номерами и куда идет каждый список сервисов.
- `/route <список> <канал> [pool|direct|block]` привязывает список к каналу. Канал задается номером из `/routes`, id, началом названия или словом `pool`, `balance`, `direct`, `block`. Последний аргумент говорит, куда идти, если канал упал. Пример: `/route youtube 2`. Изменение попадает в pending, применяется через `/param_apply`.
- `/history` (или `/failover_history`) выводит последние 20 событий failover.
- `/clients` показывает правила клиентов.
- `/logs [N]` выводит последние строки `logread` по hybrid-failover, по умолчанию 50, максимум 500.

**Управление**

- `/routing_restart` перезапускает `/etc/init.d/hybrid-failover`.
- `/switch <outbound>` или `/switch <section> <outbound>` переключает канал вручную.
- `/list_update` обновляет списки доменов.
- `/subscription_refresh` обновляет подписки.

**Failover**

- `/failover_params` показывает политику и параметры URLTest основной секции.
- `/failover_help` выводит команды редактирования failover.
- `/failover_add <uri>` добавляет резервный URI в `failover_proxy_links`, `/failover_rm <uri>` удаляет.
- `/failover_apply` применяет изменения.
- `/set_policy outage-only|prefer-primary|fastest` меняет `failover_policy`.
- `/set_urltest_interval <сек>`, `/set_urltest_tolerance <мс>`, `/set_urltest_idle_timeout <сек>`, `/set_interrupt_existing on|off` меняют параметры URLTest.
- `/set_quic on|off` меняет `settings.disable_quic`.

**Параметры UCI**

- `/params` (или `/param_list`) выводит `uci show hybrid-failover`.
- `/param_get <ключ>`, `/param_set <ключ> <значение>`, `/param_del <ключ>` читают и меняют параметр. Можно писать полный ключ `hybrid-failover.секция.опция` или алиас: `disable_quic`, `urltest_interval`, `urltest_check_interval`, `urltest_tolerance`, `urltest_idle_timeout`, `urltest_interrupt_exist_connections`, `policy`.
  Например, `/param_set urltest_tolerance 100` или `/param_get disable_quic`.
- `/uci_show [секция]`, `/uci_sections` выводят конфиг и список секций.
- `/uci_get`, `/uci_set`, `/uci_del` делают то же, что `/param_get`, `/param_set` и `/param_del`.
- `/uci_add_list <ключ> <значение>` и `/uci_del_list <ключ> <значение>` добавляют и удаляют элемент списка.
- `/param_preview` показывает изменения в pending (`uci changes`).
- `/param_apply` проверяет и применяет pending. Core сам перезагружает движок, отдельный перезапуск службы не нужен.
- `/param_rollback` отменяет pending.

**Конфиг бота**

- `/config_show` показывает `policy`, `clash_api`, `log_path` и `audit_path` из pending или основного файла.
- `/config_set <ключ> <значение>` записывает ключ в pending.
- `/config_validate` проверяет pending и показывает, отличается ли он от основного файла.
- `/config_apply` записывает pending в основной файл, после чего бот нужно перезапустить.
- `/config_rollback` удаляет pending.

Изменения UCI сначала попадают в pending, их можно посмотреть через `/param_preview` и применить или откатить. Кнопки `/param_apply`, `/param_rollback`, `/failover_apply`, `/routing_restart`, `/config_apply` и `/config_rollback` в панели просят подтверждения, оно действует 30 секунд.

## Таймауты и длинные ответы

Команда на роутере по умолчанию ограничена 30 секундами. Долгим операциям бот даёт больше времени. На перезапуск службы уходит до 2 минут, на apply до 3, на проверку каналов до 1, на обновление списков до 5 и на обновление подписок до 3 минут. Ответы длиннее лимита Telegram бот режет на несколько сообщений.

## LuCI

Страница **Сервисы → Hybrid Failover → Telegram** находится по адресу `/cgi-bin/luci/admin/services/hybrid-failover/bot`. На ней есть поля конфига бота с теми же ключами, что у `/config_set`. Сохранение пишет их в pending, отдельные кнопки проверяют, применяют и откатывают pending и перезапускают бота. Ниже идут настройки UCI `hybrid-failover-bot`, то есть включение службы и пути к бинарнику, конфигу и логу.

## Установка

Бот входит в полную установку:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

На роутер, где core уже стоит, бота можно добавить командой `HF_MODE=bot ash /tmp/install.sh`. Пакет `hybrid-failover-bot` зависит от `hybrid-failover-core`. Вкладка **Telegram** в LuCI появляется, когда бот установлен. После установки заполните `token` и `admin_ids`, затем включите бота:

```sh
uci set hybrid-failover-bot.main.enabled=1
uci commit hybrid-failover-bot
/etc/init.d/hybrid-failover-bot restart
```
