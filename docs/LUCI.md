# LuCI: руководство пользователя

Веб-интерфейс **`luci-app-hybrid-failover`** для настройки Hybrid Failover на роутере без правки UCI вручную.

**Меню:** Сервисы → Hybrid Failover  
**URL:** `http://ROUTER/cgi-bin/luci/admin/services/hybrid-failover`

Требуется установленный **`hybrid-failover-core`** (бинарник `/usr/sbin/hybrid-failover`). LuCI вызывает его через rpcd/ubus.

---

## С чего начать

1. Установите пакеты (см. [INSTALL.md](INSTALL.md)): режим `full` ставит core, LuCI и бота.
2. Выполните на роутере:
   ```sh
   hybrid-failover migrate
   /etc/init.d/hybrid-failover enable
   /etc/init.d/hybrid-failover start
   ```
3. Откройте **Сервисы → Hybrid Failover → Обзор** и убедитесь, что sing-box и nft в статусе «активны».
4. Настройте **Маршрутизацию** (VPN или proxy, списки доменов).
5. При необходимости добавьте **правила клиентов** (консоль, телевизор, телефон с другим режимом).

---

## Вкладки интерфейса

| Вкладка | Для чего |
|---------|----------|
| **Обзор** | Живой статус: sing-box, nft, Clash API, активный outbound, контроллер failover, задержки каналов, журнал переключений |
| **Маршрутизация** | Секции маршрутизации: VPN + failover, proxy URLTest, URI, подписки, community-списки, глобальные настройки DNS |
| **Диагностика** | validate, check-nft, check-fakeip, global-check, backup/restore UCI |
| **Клиенты** | Per-client правила по IP (include / exclude / full route / global exclude) |
| **Telegram** | Конфиг бота, pending validate/apply/rollback |

Landing page после установки открывает **Обзор**, не маршрутизацию.

---

## Обзор (дашборд)

Показывает, **что сейчас работает на роутере**, без редактирования конфига.

- **Карточки состояния:** sing-box, nft, Clash API, FakeIP.
- **Активный outbound:** какой канал выбран для основной секции (например `main-urltest-out`).
- **Контроллер failover:** политика, streak ошибок, последнее переключение.
- **Задержки каналов:** sparkline по данным Clash API (`delay_history`).
- **Журнал:** события из `history.jsonl` (переключения proxy/VPN).

Кнопки **Обновить**, **Health probe**, **Export history**, **Switch proxy** (ручное переключение, если политика не passive urltest).

Если дашборд пуст или «нет данных», проверьте `/etc/init.d/hybrid-failover status` и **Диагностика → global-check**.

---

## Маршрутизация

Здесь задаётся **куда уходит трафик**, который попадает под правила Hybrid Failover (community-списки, пользовательские домены/подсети, default route секции).

### Секция маршрутизации (`config section`)

Имя секции произвольное (`main`, `glob`, …). От него зависят теги в sing-box: `{section}-out`, `{section}-urltest-out`, `{section}-awg-out`.

| `connection_type` | Смысл |
|-------------------|--------|
| `vpn` | Основной путь через VPN-интерфейс (`option interface`, напр. `awgch`). Опционально резервные proxy в `failover_proxy_links` |
| `proxy` | Только proxy-ссылки (`urltest_proxy_links` или одна `proxy_string`) |
| `block` | Reject для доменов/подсетей этой секции |

**VPN + failover:** при падении VPN urltest переключается на резервные URI из списка. Политика: `failover_policy` (`outage-only`, `prefer-primary`, `fastest`).

**Списки:** `community_lists`, `user_domains_text`, `user_subnets_text` определяют, какой трафик направляется в эту секцию.

### Pending workflow на этой странице

Изменения маршрутизации проходят через **pending**:

1. **Save** → capture в `/etc/hybrid-failover/pending`
2. **Validate** (кнопка или шагпер) → проверка без применения
3. **Apply** → запись в UCI + `apply` + reload sing-box

Так можно откатить черновик (**Rollback**), не ломая рабочий конфиг.

Подробнее про UCI-опции: [UCI.md](UCI.md).

---

## Клиенты (per-client правила)

Отдельная логика: **какой LAN-клиент** (по IP) участвует в Hybrid Failover и **как именно**.

### Две области на странице

| Блок | Что это |
|------|---------|
| **Effective rules** | Read-only: что core видит после последнего reload (`list_clients`). Обновляется кнопкой **Обновить** |
| **Правила клиентов** | Редактируемые секции UCI `client_rule`: IP, режим, секция (для full route) |

Пустая таблица Effective rules **не означает**, что core сломан. Чаще всего просто **ещё нет ни одного `client_rule`**.

Сообщения:

- *«Нет правил клиентов…»* при работающем core → добавьте правило ниже.
- *«Core не запущен…»* → `/etc/init.d/hybrid-failover start`.

### Режимы `client_rule`

| Режим | Когда использовать |
|-------|---------------------|
| **Include** | Клиент **идёт через** Hybrid Failover (nft mark + tproxy + sing-box). Типичный случай для устройства, которое должно пользоваться VPN/proxy роутера |
| **Exclude** | Клиент **минует** Hybrid Failover, трафик direct (как без mark) |
| **Full route** | **Весь** трафик клиента через выбранную **секцию маршрутизации** (поле «Секция», напр. `main`) |
| **Global exclude** | Исключение из tproxy-маршрутизации глобально (direct без per-section правила) |

Legacy-списки в `settings.include_source_ips` / `exclude_source_ips` / `fully_routed_ips` core читает **только пока нет ни одной секции `client_rule`**. После появления `client_rule` используйте только вкладку **Клиенты**.

### Как добавить правило (пошагово)

1. **Выбрать из DHCP…** → в таблице leases нажать **Выбрать** у нужного устройства.  
   LuCI создаст строку правила и подставит IP (hostname виден в списке DHCP).
2. Выбрать **Режим** (чаще всего **Include** для «пустить через HF»).
3. Для **Full route** указать **Секцию маршрутизации** (имя из вкладки Маршрутизация, напр. `main`).
4. **Save & Apply** (или **Save**). Core выполнит **reload** sing-box без pending-очереди на этой странице.
5. **Обновить** в блоке Effective rules → строка с IP, режимом, источником `client_rule`.

### DHCP picker

Кнопка **Выбрать из DHCP…** читает lease-файлы dnsmasq (`/tmp/dhcp.leases` и пути из UCI).  
На типичном OpenWrt DHCP ведёт **dnsmasq**, не odhcpd.

Если список пуст:

- проверьте `/tmp/dhcp.leases` на роутере;
- убедитесь, что dnsmasq запущен;
- на странице **Status → DHCP** в стандартном LuCI leases тоже должны быть видны.

---

## Диагностика

| Действие | Что проверяет |
|----------|----------------|
| **validate** | UCI и dry-run генерации sing-box |
| **check-nft** | Таблица `inet hybrid_failover` |
| **check-fakeip** | DNS `127.0.0.42`, fakeip, HTTPS check |
| **global-check** | Сводный отчёт (sing-box, nft, clash, fakeip) |
| **backup_uci / restore** | Архив `/etc/config/hybrid-failover` |

Используйте после смены конфига или при «всё пропало с LAN».

---

## Telegram (вкладка в LuCI)

Настройка **`/etc/hybrid-failover-bot.json`** и UCI `hybrid-failover-bot`: токен, admin_ids, clash_api.

Pending workflow бота (validate / apply / rollback) аналогичен идее pending на маршрутизации, но для конфига бота.

Полный список команд бота: [bot/README.md](../bot/README.md).

---

## Сохранение: pending vs сразу

| Страница | После Save |
|----------|------------|
| **Маршрутизация** | Pending → validate → apply |
| **Клиенты** | Сразу reload sing-box |
| **Telegram** | Свой pending для конфига бота |

---

## Связанная документация

| Документ | Содержание |
|----------|------------|
| [OVERVIEW.md](OVERVIEW.md) | Архитектура, URI, DNS, failover |
| [UCI.md](UCI.md) | Все опции UCI, включая `client_rule` |
| [INSTALL.md](INSTALL.md) | Установка пакетов |
| [luci/README.md](../luci/README.md) | Исходники LuCI, rpcd, сборка пакета |
