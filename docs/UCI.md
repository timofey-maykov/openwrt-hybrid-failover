# Опции UCI (Hybrid Failover)

Файл конфигурации: **`/etc/config/hybrid-failover`**. Пакет UCI: **`hybrid-failover`**.

Секция маршрутизации: `config section 'glob'` (имя произвольное: от него теги sing-box: `glob-out`, `glob-urltest-out`, …).

Полный контекст: [OVERVIEW.md](OVERVIEW.md). Первичная настройка: `hybrid-failover migrate`.

---

## `config settings 'settings'`

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `enabled` | `0` / `1` | `1` | Включить Hybrid Failover |
| `dns_type` | string | `doh` | Тип DNS в sing-box (`doh`, `udp`, `dot`) |
| `dns_server` | string | `1.1.1.1` | Основной DNS (DoH/DoT/UDP) |
| `bootstrap_dns_server` | string | `77.88.8.8` | Bootstrap DNS для резолва DoH-хоста |
| `dns_rewrite_ttl` | string | `60` | TTL rewrite для fakeip DNS-правил |
| `cache_path` | string | `/etc/sing-box/cache.db` | Путь cache sing-box (не `/tmp/…`) |
| `config_schema_version` | int | `1` | Версия схемы после `migrate` |
| `disable_quic` | `0` / `1` | `0` | Отключить QUIC в маршрутизации |
| `dont_touch_dhcp` | `0` / `1` | `0` | Не менять dnsmasq (не перенаправлять DNS на `127.0.0.42`) |
| `clash_api_listen` | string | `127.0.0.1:9090` | Адрес Clash API (LuCI/бот; часто LAN IP) |
| `service_listen_address` | string | *(пусто)* | Legacy fallback для Clash API, если `clash_api_listen` пуст |
| `enable_yacd` | `0` / `1` | `0` | Включить Yacd UI в sing-box |
| `enable_yacd_wan_access` | `0` / `1` | `0` | Clash API на `0.0.0.0:9090` (требует `enable_yacd=1`) |
| `yacd_secret_key` | string | *(пусто)* | Bearer secret для Clash API |
| `main_section` | string | `glob` | Секция по умолчанию для `subscription-refresh` |
| `update_interval` | duration | `1d` | Интервал обновления community rulesets в sing-box |
| `download_lists_via_proxy` | `0` / `1` | `0` | Скачивать списки через proxy-секцию |
| `download_lists_via_proxy_section` | string | *(пусто)* | Имя секции для загрузки списков через proxy |
| `output_network_interface` | string | *(пусто)* | Привязка исходящего интерфейса sing-box |
| `webhook_url` | string | *(пусто)* | HTTP webhook при событиях failover (watchdog) |
| `failover_probe_interval` | duration | `30s` | Интервал фонового controller (не URLTest interval) |
| `history_max_lines` | int | `500` | Ротация `/var/log/hybrid-failover/history.jsonl` |
| `list subscription_urls` | list |: | URL подписок proxy; `hybrid-failover subscription-refresh` |
| `list include_source_ips` | list |: | IP клиентов через Hybrid Failover (nft mark) |
| `list exclude_source_ips` | list |: | IP клиентов, исключённых из Hybrid Failover |
| `list routing_excluded_ips` | list |: | IP/подсети, исключённые из tproxy-маршрутизации |

Per-client правила применяются через nft (`inet hybrid_failover`). LuCI: **Hybrid Failover → Клиенты**.

Community-списки: `hybrid-failover list-update` → `/tmp/hybrid-failover/rulesets/`, затем apply + reload sing-box при изменении hash. При `start` core скачивает списки, ставит cron по `update_interval`, перегенерирует sing-box.

---

## `config section '<name>'`

### Общие

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `connection_type` | `vpn` / `proxy` / `block` |: | Режим секции |
| `enabled` | `0` / `1` | `1` | Включить секцию |

### `connection_type 'vpn'`

| Опция | Тип | Описание |
|--------|-----|----------|
| `interface` | string | VPN-интерфейс для direct outbound (напр. `awg0`) |
| `failover_vpn_enabled` | `0` / `1` | VPN + резервные proxy через urltest |
| `list failover_proxy_links` | list | URI резервов (см. [форматы ссылок](OVERVIEW.md#поддерживаемые-форматы-ссылок-uri)) |
| `failover_policy` | string | `outage-only` (по умолчанию), `prefer-primary`, `fastest` |
| `failover_fail_threshold` | int | Серия неудачных probe primary до переключения на резервы (по умолчанию 2) |
| `failover_recover_threshold` | int | Серия успешных probe для возврата на VPN (1 для prefer-primary, 2 для outage-only) |

Без failover (`failover_vpn_enabled=0` или пустой список): один direct outbound на `interface`.

Порядок URI в `failover_proxy_links` = приоритет резервов в urltest (после основного VPN).

**Политики (`failover_policy`):**

| Значение | Поведение |
|----------|-----------|
| `outage-only` | Трафик на VPN (`interface`) пока probe успешен; при серии сбоев: резервы через urltest; возврат после 2 успешных probe |
| `prefer-primary` | Как outage-only, но возврат на VPN после 1 успешного probe |
| `fastest` | Все каналы в одном urltest, sing-box выбирает самый быстрый (старое поведение) |

Контроллер core опрашивает Clash API и переключает selector; sing-box urltest на резервах.

### `connection_type 'proxy'`

| Опция | Тип | Описание |
|--------|-----|----------|
| `proxy_config_type` | `url` / `urltest` / `outbound` | Тип proxy-конфига |
| `proxy_string` | string | Одна ссылка (при `proxy_config_type=url`) |
| `list urltest_proxy_links` | list | Список URI (при `proxy_config_type=urltest`) |
| `outbound_json` | string | Raw outbound JSON (при `proxy_config_type=outbound`) |

### `connection_type 'block'`

Секция без outbound; reject-маршрут и community rulesets.

### Списки доменов / подсетей (секция)

| Опция | Тип | Описание |
|--------|-----|----------|
| `user_domain_list_type` | `disabled` / `text` / … | Пользовательский список доменов |
| `user_domains_text` | string | Домены (при `text`) |
| `user_subnet_list_type` | `disabled` / `text` / … | Пользовательский список подсетей |
| `user_subnets_text` | string | Подсети (при `text`) |

### Общие опции URLTest

Для **VPN + failover** и **proxy urltest**:

| Опция | По умолчанию | Описание |
|--------|--------------|----------|
| `urltest_check_interval` | `3m` | Интервал проверки. Должен быть **≤** `urltest_idle_timeout` |
| `urltest_tolerance` | `50` | Допуск по задержке (ms) |
| `urltest_testing_url` | `https://www.gstatic.com/generate_204` | URL probe |
| `urltest_idle_timeout` | *(пусто)* | Таймаут простоя: `60s`, `5m` (не голое `60`) |
| `urltest_interrupt_exist_connections` | `0` | `1`: interrupt при смене узла |
| `enable_udp_over_tcp` | `0` | Для SS/SOCKS при разборе ссылок |

---

## UCI сервиса бота (`hybrid-failover-bot`)

| Опция | Описание |
|--------|----------|
| `main.enabled` | `1`: запускать бота |
| `main.binary` | Путь к `/usr/bin/hybrid-failover-bot` |
| `main.config_path` | Путь к `/etc/hybrid-failover-bot.json` |

---

## CLI и валидация

```sh
hybrid-failover migrate          # импорт прежнего UCI при необходимости, schema v1
hybrid-failover validate         # проверка UCI и dry-run apply
hybrid-failover apply            # генерация sing-box + nft
hybrid-failover check-fakeip     # dig/curl через 127.0.0.42
hybrid-failover list-update      # community domain lists
hybrid-failover subscription-refresh  # merge subscription URLs
```

При `validate`/`apply` проверяются URI, пары `urltest_check_interval` / `urltest_idle_timeout`.

---

## Пример

См. [examples/glob-uci-commands.txt](../examples/glob-uci-commands.txt) и шаблон [openwrt/etc/config/hybrid-failover](../openwrt/etc/config/hybrid-failover).
