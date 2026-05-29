# LuCI (`luci-app-hybrid-failover`)

Единый интерфейс Hybrid Failover для OpenWrt. Меню: **Сервисы → Hybrid Failover**.

Базовый URL: `/cgi-bin/luci/admin/services/hybrid-failover`

| Подраздел | Путь | Назначение |
|-----------|------|------------|
| Маршрутизация | `/routing` | VPN + failover, URLTest, subscription URLs |
| Статус | `/dashboard` | Мониторинг: карточки сервисов, каналы с задержкой, контроллер failover, журнал переключений |
| Клиенты | `/clients` | Per-client include/exclude по IP |
| Telegram-бот | `/bot` | JSON-конфиг, pending validate/apply/rollback |

Исходники: `luci/root/www/luci-static/resources/view/hybrid-failover/`, menu: `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json`, rpcd: `luci/root/usr/share/rpcd/ucode/hybrid-failover`.

## Что настраивается

1. **VPN + failover**: `failover_vpn_enabled`, `failover_proxy_links`: резервные URI при `connection_type=vpn`.
2. **Proxy URLTest**: `urltest_proxy_links` при `connection_type=proxy`.
3. **URLTest**: `urltest_check_interval`, `urltest_idle_timeout`, `urltest_interrupt_exist_connections`.
4. **Подписки**: `settings.subscription_urls` → `hybrid-failover subscription-refresh`.
5. **Per-client**: `settings.include_source_ips`, `settings.exclude_source_ips`.
6. **`vpn://` (Amnezia)**: в списках URI; декод в core (Go), Python не нужен.

## Backend

Действия apply/validate/status вызывают **`/usr/sbin/hybrid-failover`** через rpcd (`hybrid-failover rpc …`).

UCI: **`/etc/config/hybrid-failover`**. После сохранения в LuCI:

```sh
hybrid-failover validate
hybrid-failover apply
/etc/init.d/hybrid-failover restart
```

## Amnezia `vpn://`

Ссылка **`vpn://…`** декодируется core в **`vless://…`** (`internal/amnezia`). Поддерживается типичный экспорт **amnezia-xray** с VLESS в `last_config`.

## Установка

Пакет **`luci-app-hybrid-failover`** (режим `HF_MODE=full` или вместе с core):

```sh
./scripts/build-packages.sh
opkg install /tmp/luci-app-hybrid-failover_*_all.ipk
```

Или одной командой: [docs/INSTALL.md](../docs/INSTALL.md).

## Эксплуатационные заметки

- Если в логах sing-box появляется `missing fakeip record`, задайте `hybrid-failover.settings.cache_path='/etc/sing-box/cache.db'` и перезапустите core.
- Clash API: `settings.clash_api_listen` (по умолчанию `127.0.0.1:9090`; для `/health` бота часто нужен LAN IP).
- Failover вживую: `http://ROUTER:9090/proxies/<section>-urltest-out`.

## Связанная документация

- [docs/OVERVIEW.md](../docs/OVERVIEW.md): архитектура, URI, диагностика
- [docs/UCI.md](../docs/UCI.md): все опции UCI
- [bot/README.md](../bot/README.md): Telegram-бот

## Legacy

Устаревший patch-based LuCI (`legacy/section.js`) не входит в релиз. Для установок используйте **`luci-app-hybrid-failover`**.
