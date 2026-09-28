[English](en/SING-BOX-SIZE.md)

# Размер пакетов и overlay

Раньше Hybrid Failover работал поверх пакета sing-box, и на роутерах с overlay 64-128 MiB место кончалось быстро. Стоковый sing-box весил около 40 MiB. Сейчас маршрутизацию делает native engine внутри `hybrid-failover`, и sing-box больше не нужен.

## sing-box больше не ставится

Режим `engine_mode=singbox` удалён. `hybrid-failover migrate` переключает старые установки на `native`, останавливает и отключает `/etc/init.d/sing-box`, удаляет пакет `sing-box` через opkg и файл `/etc/sing-box/config.json`. На системах с apk пакет sing-box, если он остался, удалите вручную (`apk del sing-box`).

Скрипт `scripts/build-sing-box-lite.sh` пока лежит в репозитории, он собирал урезанный sing-box без tailscale, wireguard и dhcp. Для текущих версий он не нужен.

## Что занимает место сейчас

| Пакет | Что внутри | Нужен |
|-------|------------|-------|
| `hybrid-failover-core` | `/usr/sbin/hybrid-failover`, в нём engine, DNS на `127.0.0.42`, контроллер failover | всегда |
| `luci-app-hybrid-failover`, `luci-i18n-hybrid-failover` | страницы LuCI и переводы | если нужен веб-интерфейс |
| `hybrid-failover-bot` | `/usr/bin/hybrid-failover-bot` | только для Telegram |

Community-списки лежат в `/etc/hybrid-failover/rulesets/` и тоже занимают overlay. Их объём зависит от выбранных `community_lists`.

`bind-dig` и `bind-libs` не нужны. `hybrid-failover check-fakeip` проверяет DNS собственным клиентом на Go, так что эти пакеты можно снять (`opkg remove bind-dig bind-libs`).

## Сжатие бинарников UPX

`scripts/build-packages.sh` после сборки прогоняет core и бота через UPX. Поведение задаёт переменная `HF_UPX`.

| Значение | Что происходит |
|----------|----------------|
| `auto` (по умолчанию) | сжимает, если `upx` есть в `PATH`, иначе пропускает |
| `1` | сжатие обязательно, без `upx` сборка падает |
| `0` | не сжимать |

Релизная сборка в GitHub Actions ставит `upx-ucl` и собирает с `HF_UPX=1`. Скрипт печатает размер каждого бинарника до и после сжатия. Сжатый бинарник на роутере стартует чуть дольше, на доли секунды.

## Установка при нехватке места

`scripts/install-on-router.sh` смотрит свободное место на `/overlay`. Если его меньше порога или задан `HF_FORCE_REINSTALL=1`, установщик останавливает службы и снимает уже установленный пакет перед установкой нового. С opkg он ставит пакеты с `--force-space`.

| Переменная | Значение |
|------------|----------|
| `HF_LOW_SPACE_KB` | порог в КБ, по умолчанию 45000 |
| `HF_FORCE_REINSTALL` | `1` снимает и ставит заново наши пакеты при любом свободном месте |

Старые установки оставляли монолитный бинарник `/usr/bin/hybrid-failover` размером больше 6 MiB. Установщик удаляет его, если уже есть `/usr/sbin/hybrid-failover`.

## Рекомендации

1. На роутере с overlay 64 MiB ставьте только `hybrid-failover-core` и LuCI.
2. Без Telegram пакет `hybrid-failover-bot` не нужен.
3. Если после обновления со старой версии остался sing-box, проверьте, что `migrate` его удалил (`opkg list-installed | grep sing-box`).
4. Если места всё равно мало, поможет extroot.
