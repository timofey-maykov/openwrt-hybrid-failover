# LuCI (luci-app-podkop): поля под доработки

Файл **`section.js`**: форма секции маршрутизации Podkop (тип подключения, VPN, URLTest и резервные каналы).

Отдельная страница **Telegram-бота** (русский интерфейс): `luci/root/www/luci-static/resources/view/podkop/bot.js`: см. [bot/README.md](../bot/README.md).

## Что добавлено

1. **`failover_vpn_enabled`** и **`failover_proxy_links`**: сразу под **типом подключения**, при **Connection type → VPN** (список URI: при включённом failover). Так проще найти в форме и корректнее для порядка `checkDepends` в LuCI.
2. **URLTest**: интервал, tolerance, URL проверки, **`urltest_idle_timeout`**, **`urltest_interrupt_exist_connections`**: показываются и для **Proxy → URLTest**, и для **VPN + failover** (через вторую группу `depends`, логика LuCI: **ИЛИ** между группами).
3. **`UDP over TCP`**: дополнительно при **VPN + failover** (для разбора SOCKS/SS в запасных ссылках).
4. **`vpn://` (Amnezia)**: в списках ссылок и в поле proxy URL: LuCI пропускает строку; на роутере см. раздел **Amnezia `vpn://`** ниже (Python + патч `sing_box_config_facade`).

Строки переводов на английском (`_("...")`); при желании добавьте строки в шаблон **podkop.pot** / **ru.po** upstream или в свой `.lmo`.

## Amnezia `vpn://` в failover / proxy / URLTest

Ссылка **`vpn://…`** (экспорт Amnezia) на роутере превращается в **`vless://…`** скриптом **`scripts/amnezia_vpn_uri_to_vless.py`** (копия на устройство: **`/usr/lib/podkop/amnezia_vpn_uri_to_vless.py`**), затем обрабатывается как обычный VLESS.

1. **`opkg install python3-light`**
2. Скопировать скрипт:  
   `scp scripts/amnezia_vpn_uri_to_vless.py root@ROUTER:/usr/lib/podkop/amnezia_vpn_uri_to_vless.py`
3. Патч к **`/usr/lib/podkop/sing_box_config_facade.sh`**: **`patches/sing_box_config_facade-vpn-uri.patch`**  
   с хоста:  
   `scp patches/sing_box_config_facade-vpn-uri.patch root@ROUTER:/tmp/`  
   `ssh root@ROUTER 'cd / && patch -p0 --forward -i /tmp/sing_box_config_facade-vpn-uri.patch'`  
   (при повторном запуске patch может сказать «already applied»: это нормально.)
4. Обновить **`section.js`** на роутере (см. ниже) и **`/etc/init.d/podkop restart`**.

Поддерживается пока только контейнер **`amnezia-xray`** с первым outbound **`vless`** в `last_config` (как в типичном экспорте).

## Установка на роутер

Путь на устройстве: **`/www/luci-static/resources/view/podkop/section.js`**

```sh
# С хоста (если нет scp): положить файл в /tmp и перенести
base64 < luci/section.js | ssh root@ROUTER 'base64 -d > /tmp/section.js && mv /tmp/section.js /www/luci-static/resources/view/podkop/section.js'
```

Жёсткое обновление страницы в браузере (Ctrl+Shift+R). Перезагрузка `uhttpd` обычно не нужна.

Откат: восстановить оригинал из пакета:

```sh
opkg install --force-reinstall luci-app-podkop
```

## Патч

**`section.js.patch`**: отличия от stock `section.js` на момент снятия копии. Применение с ПК:

```sh
scp luci/section.js.patch root@ROUTER:/tmp/
ssh root@ROUTER 'cd / && patch -p0 -i /tmp/section.js.patch'
```

(На роутере нужен пакет **`patch`**.)

## Согласованность с backend

Поля совпадают с **`/usr/bin/podkop`** после патча **`patches/podkop-0.7.10-hybrid-urltest.patch`**. Без патча podkop UCI-опции в LuCI сохранятся, но **не повлияют** на sing-box.

## Дашборд: VPN + failover (несколько строк, как у Proxy → URLTest)

Стоковый **`main.js`** для `connection_type === 'vpn'` строит **одну** карточку и не разворачивает селектор/urltest из гибридного `podkop`. На дашборде не видно отдельно **интерфейс VPN**, **резервные URI** и строку **Fastest**, хотя sing-box уже отдаёт это в Clash API.

В каталоге **`luci/fe-app/`**:

| Файл | Назначение |
|------|------------|
| `getDashboardSections.ts` | Готовый модуль (как в [itdoginfo/podkop](https://github.com/itdoginfo/podkop) `main`, плюс ветка VPN+failover) |
| `getDashboardSections.upstream.ts` | Снимок upstream на момент синхронизации (для сравнения) |
| `getDashboardSections.vpn-failover.patch` | `diff -u` к upstream: можно наложить на свой клон |

Логика: если **`failover_vpn_enabled === '1'`** и есть **`failover_proxy_links`**, дашборд использует те же коды, что и proxy urltest: селектор **`${section}-out`**, urltest **`${section}-urltest-out`**, имена для кандидатов: индекс **0**: **`interface`** (awg0 и т.д.), дальше: **`getProxyUrlName`** по списку failover.

Сборка: в репозитории [itdoginfo/podkop](https://github.com/itdoginfo/podkop) подмените `fe-app-podkop/src/podkop/methods/custom/getDashboardSections.ts` или примените патч из `luci/fe-app/`, затем обычная сборка **fe-app-podkop** и установка обновлённого **luci-app-podkop** на роутер. Только правка **`section.js`** дашборд не меняет: нужен пересобранный **`main.js`**.

### Быстро: только дашборд на уже установленном `luci-app-podkop` v0.7.10

Патч **`patches/main-js-dashboard-vpn-failover.patch`**: вставка в `getDashboardSections` той же логики, что в `luci/fe-app/getDashboardSections.ts` (при **VPN + failover** показываются строки **Fastest**, **awg0**, резервные URI как у Proxy→URLTest).

С хоста (на роутере нет `patch`: проще залить уже пропатченный файл):

```sh
# снять текущий main.js с роутера, применить патч локально, вернуть:
curl -sS http://ROUTER/luci-static/resources/view/podkop/main.js -o /tmp/main.js
patch -p0 /tmp/main.js < patches/main-js-dashboard-vpn-failover.patch
base64 < /tmp/main.js | ssh root@ROUTER 'base64 -d > /www/luci-static/resources/view/podkop/main.js'
```

После обновления: жёсткое обновление вкладки Podkop в браузере.

Если TypeScript при сборке ругается на отсутствие полей в типе секции, добавьте в upstream **`types.ts`** для секции, например: `failover_vpn_enabled?: string` и `failover_proxy_links?: string[]`.

## Эксплуатационные заметки

- Если в логах sing-box появляется `missing fakeip record`, проверьте `cache_path`:
  - лучше использовать постоянный путь (`/etc/sing-box/cache.db`), а не `/tmp`.
  - после смены пути перезапустите `/etc/init.d/podkop restart`.
- Для проверки failover вживую можно смотреть Clash API:
  - `http://ROUTER:9090/proxies/<section>-urltest-out` (`now`, `history`, `all`).
