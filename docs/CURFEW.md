[English](en/CURFEW.md)

# Curfew

Curfew ночью ограничивает скорость выбранных устройств в LAN. В заданное время на интерфейсе LAN (по умолчанию `br-lan`) включаются правила `tc`, утром они снимаются. Скорость режется для каждого устройства отдельно, в обе стороны. Остальные устройства не ограничиваются.

Curfew состоит из двух пакетов: `curfew` (скрипты, init-скрипт, UCI) и `luci-app-curfew` (страница в LuCI). Они не зависят от Hybrid Failover. Curfew не читает конфигурацию `hybrid-failover`, не трогает маршрутизацию, nftables и engine. Пакеты лежат в том же репозитории и выходят в тех же релизах.

---

## Установка

`install-on-router.sh` не ставит curfew ни в одном режиме `HF_MODE` (`full`, `core`, `bot`). Пакеты нужно поставить вручную.

Оба пакета общие для всех архитектур (`all`). Их можно скачать со страницы [релизов на GitHub](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases). Имена файлов для версии 1.7.74:

| Пакет | OpenWrt 24.x (opkg) | OpenWrt 25.12+ (apk) |
|-------|---------------------|----------------------|
| `curfew` | `curfew_1.7.74-1_all.ipk` | `curfew-1.7.74-r1.apk` |
| `luci-app-curfew` | `luci-app-curfew_1.7.74-1_all.ipk` | `luci-app-curfew-1.7.74-r1.apk` |

### OpenWrt 24.x (opkg)

```sh
cd /tmp
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/curfew_1.7.74-1_all.ipk
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/luci-app-curfew_1.7.74-1_all.ipk
opkg update
opkg install /tmp/curfew_1.7.74-1_all.ipk
opkg install /tmp/luci-app-curfew_1.7.74-1_all.ipk
```

### OpenWrt 25.12+ (apk)

```sh
cd /tmp
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/curfew-1.7.74-r1.apk
wget https://github.com/timofey-maykov/openwrt-hybrid-failover/releases/download/v1.7.74/luci-app-curfew-1.7.74-r1.apk
apk update
apk add --allow-untrusted /tmp/curfew-1.7.74-r1.apk
apk add --allow-untrusted /tmp/luci-app-curfew-1.7.74-r1.apk
```

Пакеты не подписаны ключом OpenWrt, поэтому нужен `--allow-untrusted`.

### Зависимости

| Пакет | Зависит от |
|-------|------------|
| `curfew` | `uci`, `tc`, `kmod-sched-core`, `kmod-ifb`, `kmod-sched-connmark`, `dnsmasq` |
| `luci-app-curfew` | `luci-base`, `luci-compat`, `curfew` |

Зависимости из фидов OpenWrt пакетный менеджер ставит сам, поэтому перед установкой нужен `opkg update` или `apk update`.

### Что делает установка

- `curfew` кладёт `/usr/sbin/curfew`, `/usr/sbin/curfew-apply`, `/usr/sbin/curfew-dhcp`, `/etc/init.d/curfew`, `/etc/config/curfew` и пустой `/etc/dnsmasq.d/curfew.conf`. `/etc/config/curfew` и `/etc/dnsmasq.d/curfew.conf` объявлены как conffiles и при обновлении не перезаписываются.
- postinst `curfew` запускает `curfew-apply` и включает автозапуск `/etc/init.d/curfew`.
- postinst `luci-app-curfew` перезапускает `rpcd` и `uhttpd` и чистит кэш LuCI.

После установки curfew выключен (`enabled '0'`).

### Обновление

`hybrid-failover update apply` и кнопка обновления в LuCI Hybrid Failover обновляют `curfew` и `luci-app-curfew`, только если они уже установлены. Если пакетов нет, обновление их не ставит.

### Сборка

`scripts/build-packages.sh` собирает оба пакета вместе с остальными, если в репозитории есть каталоги `curfew/root` и `luci-curfew/root`. `HF_BUILD_SET` на них не влияет. Файлы берутся из `curfew/root` и `luci-curfew/root`, maintainer-скрипты из `packages/curfew/CONTROL` и `packages/luci-app-curfew/CONTROL`. Версия общая с Hybrid Failover (файл `VERSION`). Результат попадает в `dist/ipk`, `dist/apk` и `dist/manifest.json`.

---

## Страница в LuCI

**Сервисы → Curfew** (`admin/services/curfew`). Интерфейс страницы только на русском.

Блок **Настройки**:

| Поле | Опция UCI |
|------|-----------|
| Включено | `enabled` |
| Начало лимита | `block_time` |
| Конец лимита | `unblock_time` |
| Скорость по умолчанию | `rate_limit` |
| Burst по умолчанию | `burst` |
| Интерфейс LAN | `lan_if` |

Блок **Устройства**, одна строка на устройство:

| Поле | Опция UCI |
|------|-----------|
| Имя DHCP | `name` |
| IP | `ip` |
| Скорость | `rate_limit` |
| Burst | `burst` |

Пустые **Скорость** и **Burst** означают значения по умолчанию. **IP** в LuCI обязательное поле. Устройство без IP можно задать только через `uci`.

Блок **Статус** показывает состояние лимита, окно расписания, значения по умолчанию и список устройств с IP, скоростью, `online`/`offline` и MAC из аренды. Кнопка **Обновить** перечитывает статус. Кнопка **Sync tc** выполняет `curfew sync`.

Блок **DHCP**. Кнопка **Выбрать из DHCP** показывает текущие аренды из `/tmp/dhcp.leases`. **Выбрать** подставляет имя и IP аренды в форму устройства.

**Сохранить и применить** сохраняет UCI и запускает `curfew-apply`. После этого появляется сообщение «Настройки применены (cron, dnsmasq, tc)».

Страница работает через объект rpcd `curfew` (ucode, `/usr/share/rpcd/ucode/curfew`):

| Метод | Что делает |
|-------|------------|
| `status` | `curfew status-json`, ответ в JSON |
| `apply` | `curfew-apply` |
| `sync` | `curfew sync` |
| `dhcp_leases` | Разбирает `/tmp/dhcp.leases` |

Доступ задаёт ACL `luci-app-curfew`: чтение `status` и `dhcp_leases`, запись `apply` и `sync`, UCI `curfew`.

---

## Опции UCI

Конфигурация лежит в **`/etc/config/curfew`**, пакет UCI называется **`curfew`**.

### `config curfew`

Секция одна, анонимная. Скрипты читают `curfew.@curfew[0]`.

В колонке "По умолчанию" указано значение из шаблона `/etc/config/curfew`. Если опции нет совсем, скрипты берут другое значение, оно указано в описании.

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `enabled` | `0` / `1` | `0` | `1` включает лимит. При `0` команды `throttle` и `sync` правила не ставят |
| `block_time` | `ЧЧ:ММ` | `01:30` | Начало лимита, по времени роутера. Без опции: `00:30` |
| `unblock_time` | `ЧЧ:ММ` | `08:00` | Конец лимита. Без опции: `08:00` |
| `lan_if` | интерфейс | `br-lan` | Интерфейс, на который ставятся правила `tc` |
| `rate_limit` | скорость `tc` | `128kbit` | Скорость для устройств без своей. Формат `tc`: `64kbit`, `128kbit`, `1mbit`. Без опции: `64kbit` |
| `burst` | размер `tc` | `16k` | Burst для устройств без своего. Без опции: `16k` |

### `config device`

Одна секция на устройство. Секций может быть сколько угодно.

| Опция | Тип | По умолчанию | Описание |
|--------|-----|--------------|----------|
| `name` | string | нет | Hostname устройства из DHCP (четвёртое поле в `/tmp/dhcp.leases`). Обязательная. Секция без `name` и все секции после неё не читаются |
| `ip` | IPv4 | пусто | Адрес устройства. Если пусто, берётся IP из аренды в `/tmp/dhcp.leases` с таким же hostname |
| `rate_limit` | скорость `tc` | пусто | Своя скорость устройства. Пусто: `rate_limit` из `config curfew` |
| `burst` | размер `tc` | пусто | Свой burst. Пусто: `burst` из `config curfew` |

Пример:

```
config curfew
	option enabled '1'
	option block_time '23:00'
	option unblock_time '07:00'
	option lan_if 'br-lan'
	option rate_limit '128kbit'
	option burst '16k'

config device
	option name 'iPhone'
	option ip '192.168.1.123'
	option rate_limit '64kbit'
```

После правки через `uci` нужно применить настройки:

```sh
uci commit curfew
/etc/init.d/curfew reload
```

---

## Команды

| Команда | Что делает |
|---------|------------|
| `curfew throttle` | Ставит правила `tc`, если `enabled=1`. `curfew block` делает то же |
| `curfew unblock` | Снимает правила `tc` |
| `curfew sync` | Ставит правила, если `enabled=1` и сейчас окно лимита. Иначе снимает |
| `curfew status` | Состояние, список устройств и расписание текстом |
| `curfew status-json` | То же в JSON, для LuCI |
| `curfew-apply` | Пишет cron и `/etc/dnsmasq.d/curfew.conf`, перезапускает cron, перечитывает dnsmasq, выполняет `curfew sync` |
| `/etc/init.d/curfew start` | `curfew-apply` |
| `/etc/init.d/curfew reload` | `curfew-apply` |
| `/etc/init.d/curfew stop` | `curfew unblock` |

---

## Как это работает

### Расписание

`curfew-apply` пишет в `/etc/crontabs/root` две строки и перезапускает cron:

```
30 1 * * * /usr/sbin/curfew throttle
0 8 * * * /usr/sbin/curfew unblock
```

Старые строки curfew перед этим удаляются, остальные задачи cron остаются. Время берётся из `block_time` и `unblock_time` и считается по часовому поясу роутера.

Если роутер загрузился или настройки применили внутри окна, правила ставит `curfew sync` в конце `curfew-apply`. Окно может переходить через полночь, например `23:00`–`07:00`.

### dnsmasq

`curfew-apply` пересоздаёт `/etc/dnsmasq.d/curfew.conf`:

- для каждого устройства с известным IP строку `dhcp-host=<name>,<ip>,infinite`. Устройство с этим hostname получает тот же адрес с бессрочной арендой;
- строку `dhcp-script=/usr/sbin/curfew-dhcp`.

Затем dnsmasq перечитывает конфигурацию (`reload`, при ошибке `restart`), если служба dnsmasq включена.

dnsmasq вызывает `curfew-dhcp` при выдаче и продлении аренды (`add`, `old`). Если hostname клиента совпадает с `name` одного из устройств, выполняется `curfew sync`. Так правила обновляются, когда устройство получило адрес.

### tc

Правила ставятся на `lan_if`. Для каждого устройства:

- **отдача** (трафик от устройства): qdisc `ingress`, фильтр `flower src_ip <ip>/32` с `police rate <rate> burst <burst>`. Всё сверх лимита отбрасывается;
- **загрузка** (трафик к устройству): корневой qdisc `prio`, у каждого устройства своя полоса с `tbf rate <rate> burst <burst> latency 400ms`, фильтр `flower dst_ip <ip>/32`. Последняя полоса для остального трафика с `tbf rate 1000mbit burst 64k latency 50ms`.

Перед установкой старые корневой и ingress qdisc на `lan_if` удаляются. После успешной установки создаётся метка `/var/run/curfew.active`. По ней `status` показывает, активен ли лимит. `unblock` удаляет оба qdisc и метку.

Если хотя бы у одного устройства нет IP (не задан в UCI и нет аренды), правила не ставятся ни для кого.

---

## Проверка

```sh
curfew status
uci show curfew
grep curfew /etc/crontabs/root
cat /etc/dnsmasq.d/curfew.conf
cat /tmp/dhcp.leases
tc qdisc show dev br-lan
tc filter show dev br-lan ingress
tc filter show dev br-lan parent 1:
tc -s qdisc show dev br-lan
ls -l /var/run/curfew.active
```

| Симптом | Что проверить |
|---------|---------------|
| Лимит не включается ночью | `enabled` равно `1`. Строки curfew есть в `/etc/crontabs/root`, cron запущен (`/etc/init.d/cron status`). Время и часовой пояс роутера (`date`) |
| `curfew status` показывает `inactive` в окне | У всех устройств есть IP: в `curfew status` должны быть все устройства. Запустите `curfew sync` и посмотрите `tc qdisc show dev br-lan` |
| Устройства нет в списке | `name` совпадает с hostname в `/tmp/dhcp.leases`. Или задайте `ip` явно |
| Ошибки `tc` про `flower`, `police`, `prio` | В ядре нет нужных модулей. Проверьте `lsmod \| grep -E 'cls_flower\|act_police\|sch_prio\|sch_tbf\|sch_ingress'` |
| Устройство не получает закреплённый IP | dnsmasq читает каталог `/etc/dnsmasq.d`. Посмотрите `conf-dir` в конфигурации запущенного dnsmasq: `grep conf-dir /var/etc/dnsmasq.conf*` |
| Лимит не снимается утром | Строка `curfew unblock` есть в cron. Снять вручную: `curfew unblock` или `/etc/init.d/curfew stop` |
| Страница LuCI пустая или нет пункта меню | Перезапустите rpcd и очистите кэш: `/etc/init.d/rpcd restart; rm -rf /tmp/luci-*cache*` |

---

## Ограничения

- Только IPv4. Фильтры `tc` работают с `protocol ip`, IPv6-трафик устройства не ограничивается.
- Устройство опознаётся по hostname из DHCP. Если устройство меняет hostname или не сообщает его, задайте `ip` явно.
- Если у одного устройства нет IP, правила не ставятся ни для одного.
- Пока лимит активен, curfew занимает корневой и ingress qdisc на `lan_if`. Другие qdisc на этом интерфейсе (например SQM) удаляются при включении и при снятии лимита.
- Остальной трафик к LAN, пока лимит активен, идёт через полосу с `tbf 1000mbit`.
- По коду неотфильтрованный трафик попадает в полосы `prio` по `priomap 2 2 2 2 2 2 2 2 1 1 1 1 1 1 1 0`, а полос всего `число устройств + 1`. При одном устройстве в списке полос две, и `tc` может отклонить такой `priomap`: тогда загрузка не ограничивается, а `curfew status` показывает `inactive`. При трёх и более устройствах обычный трафик без фильтра попадает в полосу третьего устройства и тоже ограничивается. Без проблем работает ровно два устройства. Проверяйте `tc -s qdisc show dev br-lan`.
- Правила `tc` не переживают перезагрузку. При старте их ставит `/etc/init.d/curfew` (`START=99`) через `curfew sync`.
- `/etc/init.d/curfew stop` снимает правила, но строки cron остаются. При `enabled=0` они ничего не ставят.
- При удалении пакета строки cron и `/etc/dnsmasq.d/curfew.conf` не удаляются, их нужно убрать вручную.
- Аппаратное и программное ускорение (flow offloading) может пропускать часть трафика мимо `tc`.
