[English](README.en.md)

# LuCI (`luci-app-hybrid-failover`)

Веб-интерфейс Hybrid Failover для OpenWrt.

**Руководство пользователя (вкладки, кнопки, когда изменения вступают в силу):** [docs/LUCI.md](../docs/LUCI.md)

---

## Меню и пути

**Сервисы → Hybrid Failover**, базовый URL `/cgi-bin/luci/admin/services/hybrid-failover`. Корень меню ведёт на `/dashboard`.

| Путь | View |
|------|------|
| `/dashboard` | `dashboard.js` |
| `/routing` | `routing.js` |
| `/services` | `services.js` |
| `/tunnels` | `tunnels.js` |
| `/diagnostics` | `diagnostics.js` |
| `/clients` | `clients.js` |
| `/bot` | `bot.js` |
| `/update` | `update.js` |

Что на каждой вкладке, описано в [docs/LUCI.md](../docs/LUCI.md#вкладки). Названия пунктов меню заданы в `menu.d` на русском и переводятся каталогом, как остальные строки. Все пункты требуют ACL `luci-app-hybrid-failover`. Пункт `/bot` виден, только если есть исполняемый `/usr/bin/hybrid-failover-bot`.

---

## Исходники

| Путь | Содержимое |
|------|------------|
| `luci/root/www/luci-static/resources/view/hybrid-failover/` | Страницы LuCI (JS), по одной на вкладку |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js` | Общий UI: шапка страницы, карточки, таблицы, модалки, общий статус, RPC-обёртки |
| `luci/root/www/luci-static/resources/hybrid-failover/hf-charts.js` | Графики для `tunnels.js`: canvas, легенда, общий курсор, зум. Без внешних библиотек |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover.json` | Меню |
| `luci/root/usr/share/rpcd/ucode/hybrid-failover` | Backend для rpcd (ucode) |
| `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover.json` | ACL |
| `luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json`, `luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json` | Меню и ACL старого отдельного пакета `luci-app-hybrid-failover-bot` (только SDK Makefile в `packages/`, в релизы не входит) |
| `luci/po/en/hybrid-failover.po`, `luci/po/ru/hybrid-failover.po`, `luci/po/zh-cn/hybrid-failover.po` | Каталоги переводов |
| `luci/i18n/hybrid-failover.en.lmo`, `luci/i18n/hybrid-failover.zh-cn.lmo` | Скомпилированные английский и китайский каталоги |

### Переводы

Исходные строки в JS написаны на русском, поэтому русский интерфейс работает без каталога. `po/ru` почти пустой и нужен только для инструментов. Английский и упрощённый китайский каталоги покрывают почти все строки интерфейса. Строка без перевода показывается на русском. Новые строки `_('…')` добавляйте в оба каталога.

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

Ответ приходит в виде `{ ok, data }`, если core вернул JSON, иначе `{ ok, output }` или `{ ok, code, output }`. `ok` повторяет код выхода core, поэтому неудачная проверка возвращает и `ok: false`, и разобранный JSON.

### Методы ubus

| ubus | Аргументы | Команда core | Кто вызывает |
|------|-----------|--------------|--------------|
| `status` | | `rpc Status` | Обзор, Маршрутизация, Клиенты |
| `health` | | `rpc Health` | Обзор ("Проверить каналы сейчас"), таймаут 60 с |
| `history` | | `rpc History` | Обзор |
| `export_history` | | `rpc ExportHistory` | Обзор ("Экспорт журнала") |
| `delay_history` | | `rpc DelayHistory` | Обзор |
| `metrics` | | `rpc Metrics` | Объявлен в `hf-ui.js`, страницы не вызывают |
| `list_routes` | | `rpc ListRoutes` | Обзор, Маршрутизация, Сервисы и каналы |
| `validate` | | `rpc Validate` | Маршрутизация, Диагностика |
| `check_nft` | | `rpc CheckNft` | Диагностика |
| `check_fakeip` | | `rpc CheckFakeip` | Обзор, Диагностика |
| `global_check` | | `rpc GlobalCheck` | Обзор, Диагностика, таймаут 60 с |
| `switch_proxy` | `section`, `outbound` | `rpc SwitchProxy <section> <outbound>` | Обзор |
| `decode_uri` | `uri` | `rpc DecodeURI <uri>` | Маршрутизация ("Разобрать") |
| `apply` | | `hybrid-failover apply` | Маршрутизация (без изменений в форме) |
| `reload` | | `hybrid-failover reload` | Страницы не вызывают |
| `list_update` | | `rpc ListUpdate` | Маршрутизация |
| `subscription_refresh` | | `rpc SubscriptionRefresh` | Маршрутизация |
| `subscription_status` | | `rpc SubscriptionStatus` | Маршрутизация (вкладка Подписки) |
| `duplicate_section` | `from`, `to` | `rpc DuplicateSection <from> <to>` | Маршрутизация ("Копировать секцию…") |
| `list_clients` | | `rpc ListClients` | Клиенты |
| `dhcp_leases` | | Без core, см. ниже | Клиенты |
| `backup_uci` | | `rpc BackupUCI` | Диагностика |
| `backup_download` | | `rpc BackupDownload` | Диагностика |
| `restore_uci` | `path` | `rpc RestoreUCI <path>` | Диагностика |
| `pending_capture`, `pending_validate`, `pending_apply`, `pending_rollback` | | `rpc CapturePending`, `PendingValidate`, `PendingApply`, `PendingRollback` | Страницы не вызывают |
| `update_check` | | `rpc UpdateCheck` | Обновление, таймаут 100 с |
| `update_status` | | `rpc UpdateStatus` | Обновление |
| `update_apply` | | `rpc UpdateApply` | Обновление |

У `restore_uci` и `duplicate_section` пустые аргументы не передаются в core. ACL разрешает все эти методы. В `write` лежат `apply`, `reload`, `pending_capture`, `pending_apply`, `pending_rollback`, `switch_proxy`, `restore_uci`, `list_update`, `subscription_refresh`, `update_check`, `update_apply`, остальные в `read`. `duplicate_section` тоже в `read`, хотя он пишет UCI.

`dhcp_leases` читает файлы аренд dnsmasq сам (пути из `leasefile` в UCI `dhcp`, иначе `/tmp/dhcp.leases`), а если они пусты, вызывает `ubus call dhcp ipv4leases`. Вызов `luci-rpc` из rpcd приводил к deadlock, поэтому его здесь нет.

### Что идёт мимо ubus-объекта

- **Сохранение форм.** Маршрутизация, Сервисы и каналы и Клиенты пишут UCI через стандартный `uci` LuCI и применяют через `ui.changes.apply(true)`, то есть с откатом при потере связи. После commit procd запускает `hybrid-failover reload` по `procd_add_reload_trigger`. Кнопка "Сохранить и применить" на Маршрутизации без изменений в форме вызывает `validate`, затем `apply`.
- **Графики.** `tunnels.js` читает `/var/run/hybrid-failover/channel-metrics.json` и `channel-metrics-24h.json` через `fs.read_direct`.
- **Telegram.** `bot.js` читает `/etc/hybrid-failover-bot.json` через `fs.read` и вызывает `/usr/bin/hybrid-failover-bot -mode set-pending`, затем `-mode apply-config` через `fs.exec`. При ошибке вызывает `-mode rollback-config`. Бот перезапускается через ubus `rc init` (`restart`), состояние читается из `service list`. `rc init` нужен потому, что `service restart` не поднимает бота, который был выключен при загрузке.

Всё это разрешено в общем ACL. История задержек лежит в `/var/run/hybrid-failover/delay-history.json`, журнал переключений в `/var/log/hybrid-failover/history.jsonl`. У native engine нет HTTP API в духе Clash, статус и каналы приходят только через RPC.

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

Postinst пакета ставит `luci.main.rpctimeout` в 60 секунд, если значение не задано или лежит между 10 и 59 (проверка каналов и полная проверка бывают дольше стандартных 20 секунд), перезапускает rpcd и uhttpd и чистит кэш LuCI.

---

## Проверка на роутере

```sh
ubus call hybrid-failover status '{}'
hybrid-failover rpc Status
```

`scripts/luci-ubus-smoke.sh` вызывает `status`, `health`, `history`, `check_nft`, `global_check`, `export_history`, `delay_history`, `list_clients`, `dhcp_leases` и проверяет, что каждый отвечает. Потом он вызывает `switch_proxy` для секции `glob` (кроме политики `fastest`), то есть действительно переключает канал. Запускайте его на тестовом роутере.
