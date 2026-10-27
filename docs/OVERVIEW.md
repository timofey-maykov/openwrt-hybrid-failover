[English](en/OVERVIEW.md)

# Hybrid Failover: полное описание

Автономный стек маршрутизации для OpenWrt. В бинарник `hybrid-failover` встроен собственный **proxy engine** (tproxy, DNS с fakeip, outbounds, urltest). Есть поддержка Amnezia `vpn://`, расширенный URLTest, Telegram-бот и LuCI на русском.

**Core:** `/usr/sbin/hybrid-failover` · **UCI:** `/etc/config/hybrid-failover` · **nft:** `inet hybrid_failover`

---

## Содержание

1. [Архитектура](#архитектура)
2. [Режимы маршрутизации](#режимы-маршрутизации)
3. [Поддерживаемые форматы ссылок (URI)](#поддерживаемые-форматы-ссылок-uri)
4. [Опции UCI](#опции-uci)
5. [Правила для устройств](#правила-для-устройств)
6. [Уведомления о переключениях](#уведомления-о-переключениях)
7. [Диагностика](#диагностика)
8. [Типичные проблемы](#типичные-проблемы)

Установка описана в [INSTALL.md](INSTALL.md), веб-интерфейс в [LUCI.md](LUCI.md), все опции в [UCI.md](UCI.md), команды в [CLI.md](CLI.md), бот в [bot/README.md](../bot/README.md).

---

## Архитектура

![Схема Hybrid Failover](img/hybrid-failover-schema.png)

Go core `hybrid-failover` читает UCI и компилирует из него план для встроенного engine. Затем он настраивает nft tproxy (таблица `inet hybrid_failover`, mark `0x105`, порт 1602) и запускает engine внутри своего процесса. Под procd работает `hybrid-failover monitor`. В этом процессе живут engine, контроллер политики failover и watchdog.

Внешнего sing-box больше нет. Единственный режим `settings.engine_mode=native`. Старый `engine_mode=singbox` удалён, и `hybrid-failover migrate` переводит такие конфиги на native. Пакет `internal/singbox` остался в коде как набор общих констант, имён тегов и разбора списков, а также ради миграции. Трафик через него не идёт.

LuCI и бот общаются с core через `hybrid-failover rpc <Method>`. Для LuCI вызов идёт через rpcd (ucode-скрипт `/usr/share/rpcd/ucode/hybrid-failover`). Ручное переключение канала передаётся работающему engine через IPC-файлы в `/var/run/hybrid-failover`. HTTP-сервера Clash API у native engine нет, порт 9090 не открывается.

```mermaid
flowchart TB
  subgraph vpn_failover [VPN + failover]
    AWG["{section}-awg-out<br/>direct + bind_interface"]
    P1["{section}-1-out"]
    P2["{section}-2-out"]
    UT["{section}-urltest-out"]
    SEL["{section}-out<br/>selector, по умолчанию urltest"]
    AWG --> UT
    P1 --> UT
    P2 --> UT
    UT --> SEL
  end
  Client[LAN traffic] --> SEL
```

Для vpn-секции `glob` (имя произвольное, теги получают этот префикс) создаются такие outbounds. У proxy-секции с `urltest_proxy_links` набор тот же, но без `{section}-awg-out`: каналы `{section}-1-out`, `{section}-2-out` и дальше берутся из `urltest_proxy_links`.

| Outbound | Назначение |
|----------|------------|
| `{section}-awg-out` | VPN-интерфейс секции, **direct** через `option interface` (напр. `awg0`). При `outage-only` и `prefer-primary` это основной канал |
| `{section}-1-out`, `{section}-2-out`, ... | Прокси из `failover_proxy_links`, по одному на URI |
| `{section}-urltest-out` | **urltest** по всем каналам секции |
| `{section}-out` | **selector**, по умолчанию указывает на urltest |

Порядок URI в списке задаёт приоритет кандидатов после основного VPN. Внутри urltest выбирается живой и быстрый.

Все команды core перечислены в [CLI.md](CLI.md).

### DNS через 127.0.0.42

При `start` core переключает dnsmasq на DNS engine, если не задан `settings.dont_touch_dhcp=1`.

1. **dnsmasq.** Текущие настройки сохраняются в `/etc/hybrid-failover/dnsmasq-dhcp.bak`, затем ставятся `noresolv=1` и `server=127.0.0.42`.
2. **DNS engine.** Слушает `127.0.0.42:53`, раздаёт fakeip из `198.18.0.0/15` и учитывает community-списки. Остальные имена он резолвит обычным UDP-запросом к `settings.bootstrap_dns_server` (по умолчанию 77.88.8.8). Опции `dns_type` и `dns_server` native engine сейчас не использует.
3. **Проверка.** `hybrid-failover check-fakeip` спрашивает у `127.0.0.42` имя `fakeip.hybrid-failover` и ждёт адрес `198.18.x.x`.

При `stop` dnsmasq восстанавливается из backup. При загрузке init-скрипт сначала возвращает dnsmasq к обычным upstream, если там остался `127.0.0.42`, чтобы LAN не сидел без DNS, пока engine не поднялся.

```mermaid
flowchart LR
  LAN[LAN clients] --> dnsmasq
  dnsmasq -->|"127.0.0.42:53"| engineDNS[engine DNS]
  engineDNS --> fakeip[fakeip 198.18.0.0/15]
  engineDNS --> upstream["bootstrap DNS, UDP 53"]
  router[Router itself] --> dnsmasq
```

### Pending-конфиг (LuCI и бот)

LuCI и бот не коммитят UCI сразу. Они ставят изменения в staging (`uci set` без `commit`), а core ведёт снимок этих изменений в `/etc/hybrid-failover/pending`.

| Шаг | CLI | Что делает |
|-----|-----|------------|
| Capture | `pending capture` | Читает `uci changes hybrid-failover` и дописывает их в снимок |
| Validate | `pending validate` | Проверяет ключи и значения из снимка |
| Apply | `pending apply` | Коммитит staged-изменения, пересобирает план и перезагружает engine |
| Rollback | `pending rollback` | Удаляет снимок |

Снимок нужен на случай, если staging пропал, например после перезагрузки очистился `/tmp/.uci`. Тогда `pending apply` проигрывает изменения из снимка. Если staged-изменения на месте, применяются именно они, а снимок повторно не накладывается.

Те же шаги доступны как RPC-методы `CapturePending`, `PendingValidate`, `PendingApply` и `PendingRollback`. В LuCI кнопка "Сохранить" на странице маршрутизации делает capture, "Применить" вызывает `pending_apply`.

### Списки и cron

- `list-update` скачивает community-списки в `/etc/hybrid-failover/rulesets/`. Если списки изменились, core пересобирает план и обновляет engine без полного рестарта.
- При `start` core тоже запускает обновление списков.
- `update_interval` в секции `settings` (по умолчанию `1d`) задаёт cron-задачу `hybrid-failover list-update`. Задача ставится при `start`, если в конфиге есть списки, и снимается при `stop`.

---

## Режимы маршрутизации

### 1. VPN + failover

| UCI | Значение |
|-----|----------|
| `connection_type` | `vpn` |
| `failover_vpn_enabled` | `1` |
| `failover_proxy_links` | список URI (см. таблицу ниже) |
| `interface` | VPN-интерфейс, напр. `awg0` |

При `failover_policy` `outage-only` и `prefer-primary` трафик идёт через VPN-интерфейс, прокси из списка берутся, только когда он падает, а после восстановления трафик возвращается на VPN. При `fastest` основного канала нет: urltest держит самый быстрый живой, и VPN-интерфейс участвует наравне с прокси.

### 2. Proxy и URLTest

| UCI | Значение |
|-----|----------|
| `connection_type` | `proxy` |
| `proxy_config_type` | `urltest` |
| `urltest_proxy_links` | список URI |

Здесь только proxy, без `bind_interface`. Форматы URI и опции urltest те же.

### 3. Свои каналы для списков

Когда в секции несколько каналов, списки сервисов можно развести по разным туннелям, чтобы YouTube не делил канал со всем остальным. Привязки задаются на вкладке «Сервисы и каналы» или командой `/route` в боте и хранятся в `config list_route`. Список без привязки идет через пул секции, как в режимах выше. Для каждой привязки выбирается, что делать, если канал упал: уйти в пул, пойти напрямую или не пускать. Вариант `balance` раскладывает соединения списка по всем живым каналам, сайт при этом держится одного канала. Свои списки доменов и подсетей с названием (`config user_list`) привязываются так же, как готовые. Подробности в [UCI.md](UCI.md#config-list_route-name).

### Параметры URLTest

Интервал проверки, допуск по задержке и URL проверки задаются опциями `urltest_*` секции, см. [UCI.md](UCI.md#параметры-urltest). Там же отмечено, какие из них native engine пока не использует.

---

## Поддерживаемые форматы ссылок (URI)

Ссылки разбираются в Go (`internal/uri`). Amnezia `vpn://` декодирует `internal/amnezia`, Python не нужен.

### В `failover_proxy_links` и `urltest_proxy_links`

| Схема | Поддержка | Примечание |
|-------|-----------|------------|
| `vless://` | да | Reality и TLS, XTLS Vision, TCP и UDP (XUDP). Транспорт tcp, ws и grpc, остальные (httpupgrade, xhttp, h2) ссылка принимает, но канал помечается ошибкой «transport is not supported». Из параметров ссылки учитываются `sni`, `alpn`, `fp`, `allowInsecure`, `path`, `host`, `serviceName`, `authority`. Vision работает только на обычном TCP, не поверх ws и grpc |
| `ss://` | да | Shadowsocks |
| `trojan://` | да | |
| `socks4://`, `socks4a://`, `socks5://` | да | |
| `hysteria2://`, `hy2://` | да | TLS (`sni`, `insecure`), obfs, `up`/`down` в mbps |
| `vpn://` | да | Экспорт **Amnezia**, превращается в `vless://` (xray) или `awg2://` (awg, awg2) |
| `awg2://` | да (служебный URI) | Не протокол, см. [ниже](#amnezia-awg2-awg2) |
| `amneziawg://` | только в подписках | Целиком клиентский конфиг AmneziaWG в base64, так его отдают панели подписок. При загрузке подписки превращается в `awg2://`, берётся первый peer |
| `http://`, `https://` | нет | Ошибка `unsupported uri scheme` |

### Amnezia `vpn://`

Декодер встроен в core (`internal/amnezia`). Поддерживается типичный экспорт **amnezia-xray** с VLESS в `last_config`. В LuCI строку `vpn://...` можно вставлять как есть.

### Amnezia AWG2 (`awg2://`)

`awg2://` не отдельный сетевой протокол. Это служебный формат core для настройки **AmneziaWG**, включая 3.1. Core делает три вещи.

1. Создаёт интерфейс типа `amneziawg`.
2. Применяет к нему конфиг через `awg setconf`.
3. Добавляет в план engine direct outbound, привязанный к этому интерфейсу.

Строка `awg2://...` появляется при конвертации `vpn://`, если в контейнере Amnezia указан `amnezia-awg2` или `amnezia-awg`. В LuCI можно сразу вставлять `vpn://`.

Декодер переносит в query `awg2://` поля версии 3.1. Это `header_protection_key`, `content_padding_addition`, `random_trailers`, `disable_cookies`, таймеры (`rekey_after_time`, `rekey_timeout`, `reject_after_time`, `keepalive_timeout`, `max_handshake_attempts`) и `persistent_keepalive` в виде диапазона (`25-35`).

#### Пакеты на роутере

Core только пишет конфиг. Туннель поднимают `kmod-amneziawg` и `amneziawg-tools`. Для экспорта Amnezia 3.1 (`protocol_version: 3.1`, `RandomTrailers`) нужны пакеты 3.1 или новее. Они должны быть собраны под тот же OpenWrt и vermagic ядра, что стоит на роутере (`opkg info kernel`). Готовые `.ipk` часто лежат в релизах [2Grey/awg-openwrt](https://github.com/2Grey/awg-openwrt) с тегом версии OpenWrt (например `v24.10.6`).

```sh
awg --version          # должно быть 3.1.x, не 1.0.20260618
awg set --help | grep random-trailers
```

Инструменты 3.0 отклонят конфиг с ошибкой `Line unrecognized: RandomTrailers=on`, и интерфейс не поднимется.

#### Handshake и несовпадение параметров

С сервером должны совпасть `S1-S4`, `H1-H4`, `HeaderProtectionKey` и `RandomTrailers`. Опция `RandomTrailers=on` работает в обе стороны. Сервер 3.1 удлиняет пакеты handshake, клиент 3.0 их не узнаёт и молча отбрасывает. Снаружи это выглядит странно. В conntrack UDP-сессия в состоянии `ASSURED`, ответы приходят, а `awg show` показывает `0 B received` и ни одного handshake.

`DisableCookies`, `Jc`, `Jmin`, `Jmax` и `ContentPaddingAddition` совпадать не обязаны.

Старый профиль AWG 2.0 (диапазоны `H1-H4`, `I1`, без `HeaderProtectionKey`) на порту сервера 3.1 handshake не получит. Не держите такие URI в `urltest_proxy_links` рядом с 3.1, иначе watchdog будет перебирать endpoint мёртвого пира.

Один `vpn://` нельзя одновременно использовать на роутере и в приложении Amnezia на устройстве в LAN. У AWG один ключ даёт одну сессию. Телефон заберёт endpoint, и роутер останется без handshake.

```sh
awg show               # latest handshake, transfer
```

В режиме urltest карточка канала в LuCI сначала смотрит на HTTP-проверку (`urltest_testing_url`). Для AWG поверх неё учитывается handshake. Если handshake свежий, а HTTP не проходит, карточка пишет "handshake есть, HTTP urltest не проходит". Если handshake нет или он старше трёх минут, канал показан как DOWN с причиной.

Трафик идёт через selector. Пока в нём выбран urltest, живым каналом считается победитель HTTP-проверки, часто это Hysteria. Сам по себе handshake AWG urltest не переключает. Ручное "Переключить" закрепляет selector на конкретном outbound. Команда `SwitchProxy` уходит в процесс monitor через IPC и не ждёт следующего цикла опроса контроллера.

### Ссылки через Telegram-бота

Бот проверяет ссылки той же функцией, что и core (`internal/validation`). Принимаются `vless`, `trojan`, `ss`, `vpn`, `socks4`, `socks4a`, `socks5`, `hysteria2`, `hy2` и `awg2`.

---

## Опции UCI

Подробная таблица лежит в [`docs/UCI.md`](UCI.md), пример команд в [`examples/glob-uci-commands.txt`](../examples/glob-uci-commands.txt).

Конфиг хранится в `/etc/config/hybrid-failover`. `hybrid-failover migrate` запускают установщик, postinst пакета core и `hybrid-failover update apply`, вручную его запускать не нужно.

### Миграция схемы

`migrate` при первом запуске импортирует прежний UCI, если нового конфига ещё нет. Потом он доводит `settings.config_schema_version` до 5.

| Схема | Что меняется |
|-------|--------------|
| v1 | `failover_vpn_enabled=0`, если опции не было. `failover_vpn_enabled=1` для VPN-секции, где уже есть `failover_proxy_links`. `urltest_interrupt_exist_connections=0`, если не задано |
| v2 | Legacy-списки клиентов переносятся в секции `client_rule` |
| v3, v4 | `settings.engine_mode=native`, в том числе вместо `singbox` |
| v5 | `settings.disable_lan_ipv6=1`, если опция не задана |

Миграция v1 также прописывает `settings.cache_path`. Эта опция осталась от sing-box, native engine её не читает.

---

## Правила для устройств

Глобальная маршрутизация (секции, списки доменов) определяет, какой трафик попадает в engine. Правила клиентов определяют, какие устройства LAN в этом участвуют и в каком режиме.

В UCI это секции `config client_rule`, в LuCI вкладка **Клиенты**.

| `mode` | Поведение |
|--------|-----------|
| `include` | Трафик клиента идёт в engine, дальше его разбирают правила секций |
| `exclude` | DNS клиента не перехватывается, трафик по подсетям идёт мимо. FakeIP-домены всё равно идут через прокси, если клиент спрашивает DNS у роутера |
| `full_route` | Весь трафик клиента через указанную секцию (`option section`) |
| `global_exclude` | Работал только в sing-box. Native engine этот режим не обрабатывает |

Синонимы и подробности в [UCI.md](UCI.md#config-client_rule-name). Действующие правила показывает `hybrid-failover rpc ListClients`. Пустой список при работающем core значит, что правил нет. Это не ошибка.

Старые списки (`settings.include_source_ips`, `exclude_source_ips`, `routing_excluded_ips`, `fully_routed_ips` в секциях) `migrate` переносит в `client_rule`. Пока есть хотя бы один `client_rule`, старые списки не используются.

```sh
# Пример: консоль 192.168.1.50 через HF
uci set hybrid-failover.console=client_rule
uci set hybrid-failover.console.ip='192.168.1.50'
uci set hybrid-failover.console.mode='include'
uci commit hybrid-failover
hybrid-failover reload
```

Как добавить правило в веб-интерфейсе, описано в [LUCI.md](LUCI.md#клиенты).

---

## Уведомления о переключениях

Каждое переключение core пишет в `/var/log/hybrid-failover/history.jsonl` (размер задаёт `settings.history_max_lines`).

**Через бота.** Включите `notify_failover_enabled: true` в `/etc/hybrid-failover-bot.json`, бот будет присылать новые события администраторам. Подробности в [bot/README.md](../bot/README.md#уведомления-о-failover).

**Через webhook core.** Укажите HTTP endpoint в `hybrid-failover.settings.webhook_url`. При каждом автоматическом переключении core шлёт на него `POST` с JSON-телом события.

```json
{"time":"2026-09-29T10:15:00Z","section":"glob","from":"glob-awg-out","to":"glob-1-out","reason":"primary outage","policy":"outage-only"}
```

Принять такой запрос можно своим relay, который, например, перешлёт текст в Telegram через `sendMessage`. Core только делает `POST`, ответ должен быть 2xx.

---

## Диагностика

Отдельного HTTP API у engine нет. Состояние смотрят через CLI, RPC и системный лог.

```sh
hybrid-failover status          # JSON: engine, nft, fakeip, активный outbound, контроллер
hybrid-failover health          # то же плюс живая проверка каналов
hybrid-failover check-fakeip    # DNS-запрос к 127.0.0.42, без dig
hybrid-failover check-nft       # таблица inet hybrid_failover на месте
logread -e hybrid-failover
```

Остальные команды в [CLI.md](CLI.md).

В боте то же самое показывают `/status`, `/health` и `/channels`.

---

## Типичные проблемы

### Пустая таблица действующих правил на вкладке Клиенты

Core при этом может работать нормально (`engine_running: true` в статусе). Сообщение значит, что секций `client_rule` нет. Добавьте правило на вкладке **Клиенты** или через UCI, см. [LUCI.md](LUCI.md#клиенты).

### Нет leases, пустой выбор из DHCP

Список берётся из lease-файла dnsmasq (`/tmp/dhcp.leases`).

```sh
cat /tmp/dhcp.leases
/etc/init.d/dnsmasq status
```

`ubus call dhcp ipv4leases` в OpenWrt отдаёт данные, только если основной DHCPv4 это odhcpd. Leases от dnsmasq там не появятся.

### check-fakeip не проходит или у LAN нет DNS

Проверьте, что engine запущен и dnsmasq смотрит на него.

```sh
hybrid-failover status
uci -q get dhcp.@dnsmasq[0].server   # должен быть 127.0.0.42
hybrid-failover check-fakeip
logread -e hybrid-failover | tail -50
```

Если `127.0.0.42:53` занят старым экземпляром dnsmasq, помогает `/etc/init.d/hybrid-failover restart`. Core при старте сам проверяет этот случай.

### Двойной failover

Удалите сторонние скрипты автоматического failover и отключите конфликтующие init.d-сервисы маршрутизации, если переходите на Hybrid Failover. `hybrid-failover migrate` предупреждает о найденных конфликтах.

---

Устройство репозитория описано в [README.md](../README.md#структура-репозитория).
