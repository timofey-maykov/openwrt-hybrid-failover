# Опции UCI (Podkop Hybrid Failover)

Секция маршрутизации: `config section 'glob'` (имя любое: от него теги sing-box: `glob-out`, `glob-urltest-out`, …).

Полный контекст: [OVERVIEW.md](OVERVIEW.md).

## Режим `connection_type 'vpn'`

### Stock (без изменений)

- `option interface 'awg0'`: VPN-интерфейс для direct outbound
- при необходимости `domain_resolver_*`

### VPN + failover

| Опция | Тип | Описание |
|--------|-----|----------|
| `failover_vpn_enabled` | `0` / `1` | Включить цепочку: VPN-интерфейс + резервные proxy через urltest |
| `list failover_proxy_links` | list | URI резервов (см. [форматы ссылок](OVERVIEW.md#поддерживаемые-форматы-ссылок-uri)) |

Порядок элементов в списке = порядок приоритета кандидатов в urltest (после основного `{section}-awg-out`).

## Режим `connection_type 'proxy'` + `proxy_config_type 'urltest'`

| Опция | Тип | Описание |
|--------|-----|----------|
| `list urltest_proxy_links` | list | Те же URI, что и для failover, но без VPN-интерфейса |

## Общие опции URLTest

Для **VPN + failover** и **proxy urltest**:

| Опция | По умолчанию | Описание |
|--------|----------------|----------|
| `urltest_check_interval` | `3m` | Интервал проверки |
| `urltest_tolerance` | `50` | Допуск по задержке (ms) |
| `urltest_testing_url` | `https://www.gstatic.com/generate_204` | URL probe |
| `urltest_idle_timeout` | *(пусто)* | Таймаут простоя urltest |
| `urltest_interrupt_exist_connections` | `0` | `1`: interrupt при смене узла |
| `enable_udp_over_tcp` | н/п | Для SS/SOCKS при разборе ссылок |

## Глобальные настройки

| Опция | Рекомендация |
|--------|----------------|
| `podkop.settings.cache_path` | `/etc/sing-box/cache.db` (не `/tmp/…`) |
| `podkop.settings.config_schema_version` | `1` после миграции |

## Пример

См. [examples/glob-uci-commands.txt](../examples/glob-uci-commands.txt).
