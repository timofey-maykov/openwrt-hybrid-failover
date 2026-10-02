[English](README.en.md)

# LuCI (`luci-app-hybrid-failover`)

Веб-интерфейс Hybrid Failover для OpenWrt.

**Руководство пользователя (что нажимать и зачем):** [docs/LUCI.md](../docs/LUCI.md)

---

## Меню и пути

**Сервисы → Hybrid Failover**, базовый URL `/cgi-bin/luci/admin/services/hybrid-failover`. Корень меню ведёт на Обзор.

| Вкладка | Путь | View | Назначение |
|---------|------|------|------------|
| Обзор | `/dashboard` | `dashboard.js` | Статус, каналы, контроллер failover, delay history, ручное переключение |
| Маршрутизация | `/routing` | `routing.js` | Глобальные настройки, секции, URI, проверка и применение |
| Диагностика | `/diagnostics` | `diagnostics.js` | Validate, check-nft, check-fakeip, global-check, бэкап UCI |
| Клиенты | `/clients` | `clients.js` | `client_rule`, выбор из DHCP, effective rules |
| Telegram | `/bot` | `bot.js` | Служба бота, JSON-конфиг через pending |
| Обновление | `/update` | `update.js` | Проверка релизов на GitHub и установка |

---

## Исходники

| Путь | Содержимое |
|------|------------|
| `luci/root/www/luci-static/resources/view/hybrid-failover/` | Страницы LuCI (JS) |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js` | Общий UI: карточки, таблицы, модалки, RPC-обёртки |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json` | Меню |
| `luci/root/usr/share/rpcd/ucode/hybrid-failover` | Backend для rpcd (ucode) |
| `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json` | ACL |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json`, `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json` | Меню и ACL старого отдельного пакета `luci-app-hybrid-failover-bot` (только SDK Makefile в `packages/`, в релизы не входит) |
| `luci/po/en/hybrid-failover.po`, `luci/po/ru/hybrid-failover.po`, `luci/po/zh-cn/hybrid-failover.po` | Каталоги переводов |
| `luci/i18n/hybrid-failover.en.lmo`, `luci/i18n/hybrid-failover.zh-cn.lmo` | Скомпилированные английский и китайский каталоги |

### Переводы

Исходные строки в JS написаны на русском, поэтому русский интерфейс работает без каталога. `po/ru` почти пустой и нужен только для инструментов. Английский и упрощённый китайский каталоги покрывают все строки интерфейса.

После правки `.po` пересоберите каталоги вручную:

```sh
./scripts/compile-luci-i18n.sh
```

Скрипту нужен `po2lmo`. Если его нет в `PATH`, скрипт один раз клонирует `openwrt/luci` в `.cache/luci-po2lmo` и собирает его.

---

## Backend

Страницы вызывают ubus-объект `hybrid-failover`, который регистрирует ucode-скрипт rpcd. Скрипт запускает `/usr/sbin/hybrid-failover` и отдаёт его JSON обратно.

```text
LuCI (браузер) → ubus hybrid-failover.<method> → rpcd ucode → hybrid-failover rpc <Method>
```

Ответ приходит в виде `{ ok, data }`, если core вернул JSON, иначе `{ ok, code, output }`.

| ubus | Команда core |
|------|--------------|
| `status`, `health`, `history`, `export_history`, `delay_history`, `metrics` | `rpc Status`, `Health`, `History`, `ExportHistory`, `DelayHistory`, `Metrics` |
| `validate`, `check_nft`, `check_fakeip`, `global_check` | `rpc Validate`, `CheckNft`, `CheckFakeip`, `GlobalCheck` |
| `switch_proxy` (`section`, `outbound`) | `rpc SwitchProxy <section> <outbound>` |
| `decode_uri` (`uri`) | `rpc DecodeURI <uri>` |
| `apply`, `reload` | `hybrid-failover apply`, `hybrid-failover reload` |
| `list_update`, `subscription_refresh`, `duplicate_section` | `rpc ListUpdate`, `SubscriptionRefresh`, `DuplicateSection` |
| `list_clients` | `rpc ListClients` |
| `backup_uci`, `backup_download`, `restore_uci` (`path`) | `rpc BackupUCI`, `BackupDownload`, `RestoreUCI` |
| `pending_capture`, `pending_validate`, `pending_apply`, `pending_rollback` | `rpc CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` |
| `update_check`, `update_status`, `update_apply` | `rpc UpdateCheck`, `UpdateStatus`, `UpdateApply` |
| `dhcp_leases` | Без core, см. ниже |

У native engine нет HTTP API в духе Clash. Статус, каналы и история задержек приходят только через эти RPC. История задержек лежит в `/var/run/hybrid-failover/delay-history.json`, журнал переключений в `/var/log/hybrid-failover/history.jsonl`.

`dhcp_leases` читает файлы аренд dnsmasq сам (пути из `leasefile` в UCI `dhcp`, иначе `/tmp/dhcp.leases`), а если они пусты, вызывает `ubus call dhcp ipv4leases`. Вызов `luci-rpc` из rpcd приводил к deadlock, поэтому его здесь нет.

Вкладка Telegram идёт мимо этого объекта. Она вызывает `/usr/bin/hybrid-failover-bot -mode set-pending|validate-config|apply-config|rollback-config` через `fs.exec`, читает `/etc/hybrid-failover-bot.json` через `fs.read` и перезапускает бота через `service restart`. Всё это разрешено в общем ACL.

Конфиги UCI лежат в `/etc/config/hybrid-failover` и `/etc/config/hybrid-failover-bot`.

rpcd читает `/usr/share/rpcd/ucode/*` только при старте. После правки ucode-скрипта выполните `/etc/init.d/rpcd restart`.

---

## Сборка и установка

```sh
./scripts/build-packages.sh
```

Пакеты появятся в `dist/ipk` (opkg, OpenWrt 24.x) и `dist/apk` (apk, 25.12+). Формат выбирает `HF_PKG_FORMAT=ipk|apk|both`, по умолчанию `both`. `HF_BUILD_SET=core` собирает только core и бота, без LuCI.

Оба LuCI-пакета, `luci-app-hybrid-failover` и `luci-i18n-hybrid-failover`, общие для всех архитектур. Ставить их удобнее одной командой на роутере, см. [docs/INSTALL.md](../docs/INSTALL.md).

`luci-app-hybrid-failover` зависит от `luci-base`, `luci-compat`, `luci-i18n-hybrid-failover` и `hybrid-failover-core`.

Postinst пакета ставит `luci.main.rpctimeout` в 60 секунд, если значение не задано или лежит между 10 и 59 (Live probe и global-check бывают дольше стандартных 20 секунд), перезапускает rpcd и uhttpd и чистит кэш LuCI.

---

## Проверка на роутере

```sh
ubus call hybrid-failover status '{}'
hybrid-failover rpc Status
```

`scripts/luci-ubus-smoke.sh` проходит по основным ubus-методам и проверяет, что каждый отвечает.
