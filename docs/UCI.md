[English](en/UCI.md)

# Опции UCI (Hybrid Failover)

Конфигурация лежит в **`/etc/config/hybrid-failover`**, пакет UCI называется **`hybrid-failover`**. Шаблон с значениями по умолчанию находится в [openwrt/etc/config/hybrid-failover](../openwrt/etc/config/hybrid-failover).

Маршрутизацию описывают секции `config section '<имя>'`. Имя выбирается свободно, из него строятся теги outbound в engine. Для секции `glob` это `glob-out` (selector секции), `glob-urltest-out` (группа urltest), `glob-awg-out` (основной VPN-интерфейс при failover) и `glob-1-out`, `glob-2-out` и так далее для отдельных URI из списка.

Работает только native engine. Режим с внешним sing-box удалён, `hybrid-failover migrate` переводит старые установки на native. Опции, которые читал только sing-box, остались в шаблоне и в LuCI, но ни на что не влияют. Они отмечены ниже.

Общее описание в [OVERVIEW.md](OVERVIEW.md), первичная настройка через `hybrid-failover migrate`.

---

## `config settings 'settings'`

В колонке "По умолчанию" указано значение, которое берёт код, если опции нет. Если шаблон ставит другое значение, это сказано в описании.

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `engine_mode` | `native` | `native` | Любое другое значение `apply` отклоняет с просьбой запустить `migrate`. Migrate заменяет `singbox` на `native` |
| `config_schema_version` | int | `0` | Версия схемы UCI. Текущая `migrate` поднимает до `5`. В шаблоне стоит `4`, первый `migrate` после установки доводит до `5` |
| `bootstrap_dns_server` | IP | `77.88.8.8` | Upstream для DNS на `127.0.0.42`. Все A-запросы, которые не получают FakeIP, уходят сюда обычным UDP на порт 53. Этот же сервер используется при прямой загрузке community-списков |
| `dns_rewrite_ttl` | int, секунды | `60` | TTL в ответах с FakeIP. Ответы от upstream всегда идут с TTL 60 |
| `dns_type` | `doh` / `dot` / `udp` | `doh` | Попадает в план engine, но DNS-сервер native engine его сейчас не использует |
| `dns_server` | string | `1.1.1.1` | То же, что `dns_type`. В native engine сейчас не используется |
| `disable_quic` | `0` / `1` | `0` | `1` отклоняет UDP/443 к FakeIP-адресам правилом nft и отбрасывает UDP/443 внутри tproxy. Клиенты откатываются на TCP |
| `disable_lan_ipv6` | `0` / `1` | `1` | Выключает RA и DHCPv6 на LAN и ставит `filter_aaaa` в dnsmasq. Tproxy работает только с IPv4, без этого Meta и Telegram могут идти мимо прокси по IPv6. `0` возвращает сохранённые настройки LAN |
| `dont_touch_dhcp` | `0` / `1` | `0` | `1` оставляет dnsmasq как есть, без перенаправления DNS на `127.0.0.42`. Без этого перенаправления FakeIP у клиентов LAN не работает, dnsmasq придётся настроить вручную |
| `source_network_interfaces` | string | `br-lan` | Входящие интерфейсы для tproxy и перехвата DNS, через пробел. Для моста `br-*` добавляются и его порты. Опции нет в шаблоне и в LuCI |
| `main_section` | string | `glob` | Основная секция. Её показывают `status` и `health`, в неё пишет `subscription-refresh`, через неё качаются списки, если `download_lists_via_proxy_section` пуст |
| `update_interval` | `1h` / `3h` / `12h` / `1d` / `3d` | `1d` | Период cron для `hybrid-failover list-update`. Другие значения дают ошибку. Cron ставится, только если хотя бы у одной секции есть списки |
| `download_lists_via_proxy` | `0` / `1` | `0` | `1` качает community-списки через секцию. Engine поднимает для этого HTTP-прокси на `127.0.0.1:1610`. В шаблоне стоит `1` |
| `download_lists_via_proxy_section` | string | пусто | Секция для загрузки списков. Пусто значит `main_section` |
| `webhook_url` | URL | пусто | HTTP webhook на события failover. Старое имя `failover_webhook_url` читается, если `webhook_url` пуст |
| `failover_probe_interval` | duration | `30s` | Интервал фонового контроллера failover (не интервал urltest). Значения меньше `15s` заменяются на `30s`, больше `5m` обрезаются до `5m` |
| `history_max_lines` | int | `500` | Сколько строк держать в `/var/log/hybrid-failover/history.jsonl`. Допустимо от 50 до 10000, меньше 50 значит 500 |
| `delay_history_points` | int | `50` | Точек задержки на канал в `/var/run/hybrid-failover/delay-history.json`, от 10 до 200 |
| `output_network_interface` | string | пусто | Читается в план engine, но native engine его не применяет |
| `list subscription_urls` | list | - | URL подписок. `hybrid-failover subscription-refresh` или LuCI записывают ссылки в `main_section` |
| `list include_source_ips` | list | - | Устаревшее, см. `client_rule` |
| `list exclude_source_ips` | list | - | Устаревшее, см. `client_rule` |
| `list routing_excluded_ips` | list | - | Устаревшее, см. `client_rule` с `mode global_exclude` |

### Опции без эффекта в native engine

Эти опции есть в шаблоне и в LuCI, но native engine их не использует. Все, кроме `enabled`, читает только код legacy sing-box и поиск адреса Clash API. У native engine нет HTTP-сервера Clash API, на `:9090` ничего не слушает.

| Опция | Шаблон | Что делала раньше |
|--------|--------|-------------------|
| `enabled` | `1` | Флаг в LuCI. Код его не читает, службу включают через `/etc/init.d/hybrid-failover enable` |
| `cache_path` | `/etc/sing-box/cache.db` | Файл cache sing-box. `migrate` по-прежнему прописывает его, если опции нет |
| `clash_api_listen` | `127.0.0.1:9090` | Адрес Clash API sing-box |
| `service_listen_address` | пусто | Запасной адрес Clash API, если `clash_api_listen` пуст |
| `enable_yacd` | `0` | Веб-интерфейс Yacd в sing-box |
| `enable_yacd_wan_access` | `0` | Clash API на `0.0.0.0:9090` |
| `yacd_secret_key` | пусто | Bearer secret для Clash API |

---

## `config client_rule '<name>'`

Правила для отдельных клиентов LAN. Правило задаёт IP клиента и то, как обрабатывается его трафик. В LuCI это вкладка **Hybrid Failover → Клиенты**, пошагово в [LUCI.md](LUCI.md).

| Опция | Тип | Описание |
|--------|-----|----------|
| `ip` | string | IP или CIDR клиента, например `192.168.11.236`. Правило без `ip` пропускается |
| `mode` | string | Режим, см. таблицу ниже |
| `section` | string | Секция маршрутизации. Нужна только для `full_route` |

### Режимы `mode`

| Значение | Синонимы | Что происходит |
|----------|----------|----------------|
| `include` | `in` | Весь трафик клиента помечается и идёт в tproxy. Дальше его разбирают правила engine |
| `exclude` | `out` | DNS клиента не перехватывается, трафик не попадает в tproxy по подсетям. FakeIP-адреса community-доменов всё равно идут через прокси, если клиент спрашивает DNS у роутера. Консоли Xbox и PS сюда не ставьте, для них есть `subnet_bypass_ips` |
| `full_route` | `full`, `fully_routed` | Трафик клиента помечается как у `include`, а engine отправляет всё от этого IP в секцию `section`, раньше правил по спискам |
| `global_exclude` | `routing_excluded` | Работал только в sing-box, как исключение адресов из маршрутизации. Native engine этот режим не обрабатывает |

После изменения правил LuCI сам делает reload, из консоли нужен `hybrid-failover reload`.

Проверка через `hybrid-failover rpc ListClients` или кнопку **Обновить** на вкладке **Клиенты** (таблица Effective rules).

### Устаревшие списки

Пока в конфиге нет ни одной секции `client_rule`, core читает старые списки.

| UCI | Режим |
|-----|-------|
| `settings.include_source_ips` | `include` |
| `settings.exclude_source_ips` | `exclude` |
| `settings.routing_excluded_ips` | `global_exclude` |
| `section.<имя>.fully_routed_ips` | `full_route` для этой секции |

`hybrid-failover migrate` (схема v2) переносит их в `client_rule`. Как только появилась хоть одна секция `client_rule`, старые списки больше не читаются. Новые правила добавляйте через `client_rule` или вкладку **Клиенты**.

---

## `config section '<name>'`

### Общие

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `connection_type` | `vpn` / `proxy` / `block` | - | Тип секции |
| `enabled` | `0` / `1` | `1` | `0` убирает секцию из плана engine. Правила nft из её `udp_routed_ips`, `subnet_bypass_ips` и подсетей при этом остаются |

### `connection_type 'vpn'`

| Опция | Тип | Описание |
|--------|-----|----------|
| `interface` | string | VPN-интерфейс, например `awg0`. Обязателен |
| `failover_vpn_enabled` | `0` / `1` | Включает резервные proxy для VPN |
| `list failover_proxy_links` | list | URI резервов. Форматы описаны в [OVERVIEW.md](OVERVIEW.md) в разделе про поддерживаемые ссылки |
| `failover_policy` | string | `outage-only` (по умолчанию), `prefer-primary` или `fastest`. `latency` и `urltest` понимаются как `fastest` |
| `failover_fail_threshold` | int | Сколько неудачных проверок VPN подряд нужно для перехода на резервы. По умолчанию 2 |
| `failover_recover_threshold` | int | Сколько удачных проверок подряд нужно для возврата на VPN. По умолчанию 1 для `prefer-primary` и 2 для остальных |

Если `failover_vpn_enabled=0` или список резервов пуст, секция получает один outbound с привязкой к `interface`.

Порядок URI в `failover_proxy_links` задаёт порядок резервов в urltest после основного VPN.

**Политики `failover_policy`**

| Значение | Поведение |
|----------|-----------|
| `outage-only` | Трафик идёт через `interface`, пока проверки проходят. После серии сбоев контроллер переключает selector на urltest по резервам. Возврат на VPN после 2 удачных проверок |
| `prefer-primary` | Как `outage-only`, но возврат после 1 удачной проверки |
| `fastest` | VPN и резервы в одном urltest, engine выбирает самый быстрый канал. Контроллер failover в переключение не вмешивается |

Контроллер работает в фоне core, проверяет каналы и переключает selector через управляющий канал engine. Clash API для этого не нужен.

### `connection_type 'proxy'`

| Опция | Тип | Описание |
|--------|-----|----------|
| `proxy_config_type` | `url` / `urltest` / `outbound` | Тип конфигурации, по умолчанию `url` |
| `proxy_string` | string | Одна ссылка, при `proxy_config_type=url` |
| `list urltest_proxy_links` | list | Список URI, при `proxy_config_type=urltest`. Несколько `awg2://` с одним `public_key` и разными IP дают один интерфейс с ротацией endpoint. Для Amnezia 3.1 нужны kmod и tools версии 3.1 или новее, иначе `RandomTrailers` ломает handshake. Не смешивайте в одном списке мёртвый AWG 2.0 и живой 3.1 |
| `outbound_json` | string | JSON outbound, при `proxy_config_type=outbound`. Native engine принимает только `"type": "socks"` (поля `server`, `server_port`) и `"type": "direct"` (поле `bind_interface`) |

### `connection_type 'block'`

Секция без outbound. Трафик, совпавший с её списками, engine отклоняет.

### Списки доменов и подсетей

Если у секции задан хотя бы один список, она работает по спискам. Engine отправляет в неё только совпавший трафик, всё остальное уходит напрямую. Секция `vpn` или `proxy` без списков забирает весь трафик, который попал в tproxy и не совпал с другими правилами.

| Опция | Тип | Описание |
|--------|-----|----------|
| `list community_lists` | list | Готовые списки [itdoginfo/allow-domains](https://github.com/itdoginfo/allow-domains), например `russia_inside`, `youtube`, `discord`, `telegram`, `meta`. Домены получают FakeIP. Для `twitter`, `meta`, `discord`, `roblox`, `telegram`, `cloudflare`, `hetzner`, `ovh`, `digitalocean` и `cloudfront` дополнительно качаются IPv4-подсети. Файлы лежат в `/etc/hybrid-failover/rulesets/` |
| `user_domain_list_type` | `disabled` / `dynamic` / `text` | Источник своих доменов |
| `list user_domains` | list | Домены при `dynamic` |
| `user_domains_text` | string | Домены при `text`, через перевод строки, пробел или запятую |
| `list local_domain_lists` | list | Пути к файлам со списками доменов на роутере |
| `user_subnet_list_type` | `disabled` / `dynamic` / `text` | Источник своих подсетей |
| `list user_subnets` | list | Подсети при `dynamic` |
| `user_subnets_text` | string | Подсети при `text`, разделители те же |
| `list local_subnet_lists` | list | Файлы с подсетями (`.lst` или JSON ruleset) |
| `list remote_domain_lists` | list | URL списков доменов. Native engine их не качает и не применяет |
| `list remote_subnet_lists` | list | URL списков подсетей в формате `.lst` |
| `list subnet_bypass_ips` | list | IP клиентов, обычно консолей. FakeIP-трафик и community-домены идут через прокси, а подсети из `hf_proxy_subnets` и Teredo остаются на WAN. IP лучше закрепить статической арендой DHCP |
| `list udp_routed_ips` | list | IP клиентов, у которых весь UDP, кроме DNS, уходит в tproxy и в эту секцию. TCP идёт напрямую. Работает для секций `vpn` и `proxy` |

Свои подсети (`user_subnets`, `user_subnets_text`, `local_subnet_lists`, `remote_subnet_lists`) попадают в набор nft `hf_proxy_subnets`, и трафик к ним уходит в tproxy. Правило маршрута в native engine для них сейчас не строится, поэтому такой трафик engine отправляет напрямую. Подсети из `community_lists` маршрутизируются в секцию как положено.

### Параметры urltest

Используются в VPN с резервами и в proxy с `proxy_config_type=urltest`.

| Опция | По умолчанию | Описание |
|--------|--------------|----------|
| `urltest_check_interval` | `3m` | Интервал проверки. Указывайте с единицей (`30s`, `3m`), число без единицы engine не разберёт и возьмёт `30s`. Не должен быть больше `urltest_idle_timeout`. В шаблоне стоит `30s` |
| `urltest_tolerance` | `50` | Допуск по задержке в мс. Узел меняется, только если новый быстрее на это значение |
| `urltest_testing_url` | `https://www.gstatic.com/generate_204` | URL для проверки |
| `urltest_idle_timeout` | пусто | Проверяется только пара с `urltest_check_interval` при `validate` и `apply`. Сам engine это значение не использует |
| `urltest_interrupt_exist_connections` | `0` | Попадает в план, но native engine не рвёт существующие соединения при смене узла |

### Опции секции без эффекта в native engine

| Опция | Что делала раньше |
|--------|-------------------|
| `enable_udp_over_tcp` | UDP over TCP для SS и SOCKS в sing-box. Native engine опцию читает и игнорирует |
| `domain_resolver_enabled` | Отдельный DNS для имени сервера outbound в sing-box |
| `domain_resolver_dns_type` | Тип этого DNS (`doh`, `dot`, `udp`) |
| `domain_resolver_dns_server` | Адрес этого DNS |
| `list fully_routed_ips` | Заменён на `client_rule` с `mode full_route`, читается только пока нет `client_rule` |

---

## UCI сервиса бота (`/etc/config/hybrid-failover-bot`)

Секция `config bot 'main'`. Эти опции читает init-скрипт `/etc/init.d/hybrid-failover-bot`.

| Опция | По умолчанию | Описание |
|--------|--------------|----------|
| `enabled` | `0` | `1` запускает бота |
| `binary` | `/usr/bin/hybrid-failover-bot` | Путь к бинарнику |
| `config_path` | `/etc/hybrid-failover-bot.json` | Путь к JSON-конфигу бота |
| `log_path` | `/var/log/hybrid-failover-bot.log` | Файл журнала, передаётся боту через `HF_BOT_LOG_PATH` |

---

## CLI и проверка

```sh
hybrid-failover migrate [--dry-run]    # импорт старого UCI и миграция схемы до v5
hybrid-failover validate [--dry-run]   # проверка UCI и сборка плана engine
hybrid-failover apply [--dry-run]      # применить план engine и nft, reload при изменениях
hybrid-failover reload                 # перечитать UCI
hybrid-failover check-fakeip           # DNS через 127.0.0.42 без bind-dig
hybrid-failover list-update            # обновить community-списки
hybrid-failover subscription-refresh   # записать ссылки из подписок в main_section
```

`validate` и `apply` проверяют URI в `failover_proxy_links` и `urltest_proxy_links` и пару `urltest_check_interval` / `urltest_idle_timeout`.

---

## Пример

См. [examples/glob-uci-commands.txt](../examples/glob-uci-commands.txt) и шаблон [openwrt/etc/config/hybrid-failover](../openwrt/etc/config/hybrid-failover).
