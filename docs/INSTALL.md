[English](en/INSTALL.md)

# Установка на OpenWrt

## Одна команда на роутере

Пакеты берутся из [релизов на GitHub](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases). Для каждого релиза там лежат `.ipk` для OpenWrt 24.x и `.apk` для 25.12+.

```sh
# OpenWrt 24.x (opkg):
opkg update && opkg install curl ca-bundle wget
# OpenWrt 25.12+ (apk):
# apk update && apk add curl ca-bundle wget

wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh

ash /tmp/install.sh
```

Скрипт сам определяет менеджер пакетов (`apk` или `opkg`) и архитектуру из `/etc/openwrt_release`, скачивает подходящие файлы и ставит их. Потом он чистит кэш LuCI и перезапускает `rpcd` и `uhttpd`. В режимах `full` и `core` он ещё выполняет `hybrid-failover migrate`, включает и запускает службу `hybrid-failover` и проверяет конфиг через `hybrid-failover validate`. Отдельно запускать `migrate` и `start` после скрипта не нужно.

Внешний пакет sing-box не нужен. Движок встроен в `hybrid-failover-core`.

### Режимы установки

| `HF_MODE` | Что устанавливается |
|-----------|---------------------|
| `full` (по умолчанию) | `hybrid-failover-core`, `luci-i18n-hybrid-failover`, `luci-app-hybrid-failover`, `hybrid-failover-bot` |
| `core` | то же без бота |
| `bot` | только `hybrid-failover-bot` |

Режим задаётся переменной окружения, например так.

```sh
HF_MODE=core ash /tmp/install.sh
```

Пакет бота зависит от `hybrid-failover-core`, поэтому режим `bot` рассчитан на роутер, где core уже стоит. LuCI-страницы бота в этом режиме не будет, отдельного пакета для неё больше нет, а `luci-app-hybrid-failover` требует core. Службу `hybrid-failover` скрипт в режиме `bot` не трогает. В режиме `core` вкладка **Telegram** в LuCI скрыта, пока не установлен `/usr/bin/hybrid-failover-bot`.

### Переменные окружения

```sh
HF_REPO=timofey-maykov/openwrt-hybrid-failover   # репозиторий GitHub
HF_VERSION=latest            # или конкретный тег, например v1.7.48
HF_BRANCH=main               # ветка, из которой читается VERSION, если API GitHub недоступен
HF_TOKEN=123456789:ABC...    # токен бота, сразу записывается в /etc/hybrid-failover-bot.json
HF_ADMIN_IDS=123456789       # только подсказка в логе, admin_ids в JSON не записываются
HF_LOW_SPACE_KB=45000        # порог свободного места на /overlay в КБ
HF_FORCE_REINSTALL=1         # снять старый пакет перед установкой нового
HF_DIST_DIR=/tmp/hf-dist     # каталог с локальными пакетами (ipk/ и apk/)
```

Если в `HF_DIST_DIR` есть подкаталог `ipk/` или `apk/` под текущий менеджер пакетов, скрипт ставит пакеты оттуда и в GitHub не ходит.

### После установки

1. Если бот установлен, отредактируйте `/etc/hybrid-failover-bot.json`, минимум `token` и `admin_ids`. То же можно сделать в LuCI на вкладке **Telegram**. Остальные ключи описаны в [bot/README.md](../bot/README.md).
2. Включите бота. После установки он выключен:
   ```sh
   uci set hybrid-failover-bot.main.enabled=1
   uci commit hybrid-failover-bot
   /etc/init.d/hybrid-failover-bot restart
   ```
3. Откройте LuCI, **Сервисы → Hybrid Failover**
   (`http://ROUTER/cgi-bin/luci/admin/services/hybrid-failover`).
   Обычный порядок такой. **Обзор** показывает статус, на **Маршрутизации** задаются секции и URI, на **Клиентах** правила по IP. Подробно в [LUCI.md](LUCI.md).
4. В Telegram откройте своего бота и отправьте `/panel`.

## Ручная установка пакетов

Пакеты можно скачать из релиза или собрать на ПК:

```sh
./scripts/build-packages.sh
```

По умолчанию собираются и `.ipk`, и `.apk` в `dist/ipk/` и `dist/apk/`. Подробности сборки в [packages/README.md](../packages/README.md).

Архитектуру роутера покажет команда:

```sh
. /etc/openwrt_release && echo "$DISTRIB_ARCH"
```

Релизы собираются для `aarch64_cortex-a53`, `aarch64_generic`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc` и `x86_64`. LuCI-пакеты общие для всех архитектур.

### OpenWrt 24.x (opkg, `.ipk`)

```sh
opkg install /tmp/hybrid-failover-core_1.7.48-1_aarch64_cortex-a53.ipk
opkg install /tmp/hybrid-failover-bot_1.7.48-1_aarch64_cortex-a53.ipk
opkg install /tmp/luci-i18n-hybrid-failover_1.7.48-1_all.ipk
opkg install /tmp/luci-app-hybrid-failover_1.7.48-1_all.ipk
```

### OpenWrt 25.12+ (apk, `.apk`)

С 25.12 вместо opkg используется apk (Alpine Package Keeper). Имена файлов строятся как `пакет-версия-rN_arch.apk`, у пакетов без архитектуры суффикса нет.

```sh
apk add --allow-untrusted /tmp/hybrid-failover-core-1.7.48-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/hybrid-failover-bot-1.7.48-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/luci-i18n-hybrid-failover-1.7.48-r1.apk
apk add --allow-untrusted /tmp/luci-app-hybrid-failover-1.7.48-r1.apk
```

Пакеты не подписаны ключом OpenWrt, поэтому нужен `--allow-untrusted`.

### Первый запуск после ручной установки

Скрипт `postinst` пакета core создаёт `/etc/hybrid-failover/pending` и выполняет `migrate`. Службу нужно включить и запустить самому:

```sh
/etc/init.d/hybrid-failover enable
/etc/init.d/hybrid-failover start
```

`hybrid-failover migrate` доводит UCI до текущей схемы. Он импортирует старый конфиг, если своего ещё нет, выключает старый init-скрипт, переводит `engine_mode` на `native` и предупреждает о конфликтующих скриптах. Внешний sing-box он останавливает и выключает, а на opkg ещё и удаляет пакет. Команду можно запускать повторно, при отсутствии изменений она пишет `migration: no changes`. С `--dry-run` она только показывает план.

## Установка с ПК по SSH

```sh
./scripts/install-from-local-dist.sh 192.168.42.1   # копирует dist/ipk и dist/apk в /tmp/hf-dist и запускает установщик
./scripts/deploy-telegram-bot.sh 192.168.42.1       # только бинарник бота и его конфиги, для разработки
```

Пользователь SSH по умолчанию `root`, другой задаётся через `ROUTER_USER`. `deploy-telegram-bot.sh` собирает бота под `mipsle` (softfloat), для других архитектур соберите бинарник сами. Скрипт копирует и `/etc/hybrid-failover-bot.json` из репозитория, так что конфиг на роутере будет перезаписан.

## Зависимости

| Пакет | Depends |
|-------|---------|
| `hybrid-failover-core` | `ca-bundle`, `uci`, `procd` |
| `hybrid-failover-bot` | `libc`, `procd`, `ca-bundle`, `uci`, `hybrid-failover-core` |
| `luci-app-hybrid-failover` | `luci-base`, `luci-compat`, `luci-i18n-hybrid-failover`, `hybrid-failover-core` |
| `luci-i18n-hybrid-failover` | `luci-base` |

Установщик дополнительно ставит `curl`, `ca-bundle` и `wget` из фидов OpenWrt. Остальные зависимости менеджер пакетов подтягивает из фидов сам.

## AmneziaWG 3.1

`kmod-amneziawg` и `amneziawg-tools` в релизы не входят и ставятся отдельно. Они нужны, если в urltest или failover есть `vpn://` или `awg2://` с параметрами Amnezia 3.1 (`RandomTrailers`, `HeaderProtectionKey`).

| Пакет | Требование |
|-------|------------|
| `kmod-amneziawg` | 3.1 или новее, vermagic совпадает с ядром роутера |
| `amneziawg-tools` | 3.1 или новее, в `awg set --help` есть `random-trailers` |

Сборка должна соответствовать `DISTRIB_RELEASE` и версии ядра (`opkg info kernel` или `apk info kernel`). Готовые пакеты можно взять в релизах [2Grey/awg-openwrt](https://github.com/2Grey/awg-openwrt), теги там называются по версии OpenWrt, например `v24.10.6`. Выбирайте файлы под свою цель и архитектуру, например `ramips/mt7621` и `mipsel_24kc`.

`amneziawg-tools` зависит от виртуального пакета `ip`. На образе, где `ip` даёт busybox, поставьте `ip-tiny` или `ip-full` либо установите tools с `--force-depends`.

Handshake и поля URI описаны в [OVERVIEW.md](OVERVIEW.md#amnezia-awg2-awg2).

## Релизы и обновление

Тег `v*` запускает [workflow сборки](../.github/workflows/release.yml). Он собирает `.ipk` и `.apk` для всех архитектур и публикует их в [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) вместе с `manifest.json`, где записаны sha256 и размер каждого файла. Пуши в `main` только проверяют, что пакеты собираются.

Установленную систему можно обновить с роутера:

```sh
hybrid-failover update check     # последний релиз на GitHub и есть ли обновление
hybrid-failover update apply     # поставить последний релиз, работает в фоне
hybrid-failover update status    # ход и журнал последнего обновления
```

`apply` скачивает `hybrid-failover-core`, `luci-i18n-hybrid-failover` и `luci-app-hybrid-failover`, а также `hybrid-failover-bot`, `curfew` и `luci-app-curfew`, если они уже стоят. Каждый файл сверяется с `manifest.json`. После установки выполняются `migrate` и перезапуск `rpcd`, `hybrid-failover` и бота. Журнал пишется в `/tmp/hybrid-failover/update/update.log`. `--foreground` запускает обновление в текущей консоли, `--tag v1.7.48` ставит конкретный релиз, `--force` разрешает ту же или более старую версию. Переменная `HF_REPO` меняет репозиторий.

В LuCI то же самое на вкладке **Обновление**, см. [LUCI.md](LUCI.md#обновление).

## Мало места на overlay

Бинарники в релизах сжаты UPX. Если на `/overlay` свободно меньше `HF_LOW_SPACE_KB` (по умолчанию 45000 КБ), установщик останавливает службы, снимает уже установленный пакет и ставит новый на его место. То же поведение включается принудительно:

```sh
HF_FORCE_REINSTALL=1 ash /tmp/install.sh
```
