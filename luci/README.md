# LuCI (`luci-app-hybrid-failover`)

Веб-интерфейс Hybrid Failover для OpenWrt.

**Руководство пользователя (что нажимать и зачем):** [docs/LUCI.md](../docs/LUCI.md)

---

## Меню и пути

**Сервисы → Hybrid Failover** · базовый URL: `/cgi-bin/luci/admin/services/hybrid-failover`

| Вкладка | Путь | Назначение |
|---------|------|------------|
| Обзор | `/dashboard` | Статус, failover, delay history |
| Маршрутизация | `/routing` | Секции, URI, pending apply |
| Диагностика | `/diagnostics` | validate, global-check, backup |
| Клиенты | `/clients` | `client_rule`, DHCP picker, effective rules |
| Telegram | `/bot` | Конфиг бота, pending |

---

## Исходники

| Путь | Содержимое |
|------|------------|
| `luci/root/www/luci-static/resources/view/hybrid-failover/` | Страницы LuCI (JS) |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js` | Общий UI: карточки, таблицы, модалки, RPC |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json` | Меню |
| `luci/root/usr/share/rpcd/ucode/hybrid-failover` | rpcd/ubus backend |
| `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json` | ACL |
| `luci/po/en/hybrid-failover.po` | i18n (EN) |

Компиляция переводов: `./scripts/compile-luci-i18n.sh`

---

## Backend

Действия LuCI вызывают **`/usr/sbin/hybrid-failover`** через rpcd:

```text
LuCI (browser) → ubus hybrid-failover.* → rpcd ucode → hybrid-failover rpc …
```

Примеры ubus-методов: `status`, `apply`, `reload`, `list_clients`, `dhcp_leases`, `pending_apply`.

UCI: **`/etc/config/hybrid-failover`**.

**Клиенты:** `dhcp_leases` читает lease-файлы dnsmasq напрямую (не `luci-rpc` из rpcd, иначе deadlock).

---

## Сборка и установка

```sh
./scripts/build-packages.sh
opkg install /tmp/luci-app-hybrid-failover_*_all.ipk
```

Или одной командой на роутере: [docs/INSTALL.md](../docs/INSTALL.md).

Зависимости пакета: `luci-base`, `luci-compat`, `luci-i18n-hybrid-failover`, `hybrid-failover-core`.

---

## Эксплуатация

- FakeIP: `hybrid-failover.settings.cache_path='/etc/sing-box/cache.db'`
- Clash API: `settings.clash_api_listen` (для бота часто LAN IP, не только `127.0.0.1`)
- Failover live: `http://ROUTER:9090/proxies/<section>-urltest-out`

---

## Legacy

Patch-based LuCI (`legacy/section.js`) не входит в релиз. Используйте **`luci-app-hybrid-failover`**.
