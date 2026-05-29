# Установка на OpenWrt

## Одна команда на роутере

После публикации [релиза](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) на GitHub:

```sh
# OpenWrt 24.x (opkg):
opkg update && opkg install curl ca-bundle wget
# OpenWrt 25.12+ (apk):
# apk update && apk add curl ca-bundle wget

wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh

ash /tmp/install.sh
```

По умолчанию ставится **полный набор**: `hybrid-failover-core`, Telegram-бот и LuCI.

### Режимы установки

| `HF_MODE` | Что устанавливается |
|-----------|---------------------|
| `full` (по умолчанию) | `hybrid-failover-core` + `hybrid-failover-bot` + `luci-app-hybrid-failover` |
| `bot` | Только `hybrid-failover-bot` и `luci-app-hybrid-failover` |

### Переменные окружения

```sh
HF_REPO=timofey-maykov/openwrt-hybrid-failover   # репозиторий GitHub
HF_VERSION=latest                               # или v1.0.1, v1.0.2, …
HF_BRANCH=main                                  # ветка для fallback-файлов
HF_TOKEN=123456789:ABC...                       # опционально: токен в JSON на роутере
HF_ADMIN_IDS=123456789                          # ваш Telegram user ID (подсказка в логе)
```

### Миграция и первый запуск

```sh
hybrid-failover migrate
/etc/init.d/hybrid-failover enable
/etc/init.d/hybrid-failover start
```

`migrate` при необходимости импортирует прежний UCI, выставляет `config_schema_version=1` и `cache_path`, предупреждает о конфликтующих скриптах.

### После установки

1. Отредактируйте `/etc/hybrid-failover-bot.json`: `token`, `admin_ids`, `clash_api` (часто `http://192.168.x.1:9090`, не `127.0.0.1`).
2. Включите сервис бота:
   ```sh
   uci set hybrid-failover-bot.main.enabled=1
   uci commit hybrid-failover-bot
   /etc/init.d/hybrid-failover-bot restart
   ```
3. Откройте LuCI: **Сервисы → Hybrid Failover**  
   `http://ROUTER/cgi-bin/luci/admin/services/hybrid-failover`
4. В Telegram откройте своего бота и отправьте `/panel`.

## Ручная установка пакетов

На ПК соберите пакеты (по умолчанию и `.ipk`, и `.apk`):

```sh
./scripts/build-packages.sh
```

### OpenWrt 24.x (opkg / `.ipk`)

```sh
. /etc/openwrt_release && echo "$DISTRIB_ARCH"

opkg install /tmp/hybrid-failover-core_*_aarch64_cortex-a53.ipk
opkg install /tmp/hybrid-failover-bot_*_aarch64_cortex-a53.ipk
opkg install /tmp/luci-app-hybrid-failover_*_all.ipk
```

### OpenWrt 25.12+ (apk / `.apk`)

С [25.12](https://openwrt.org/) вместо opkg используется **apk** (Alpine Package Keeper, не Android). Имена: `пакет-версия-rN[_arch].apk`.

```sh
apk add --allow-untrusted /tmp/hybrid-failover-core-1.0.5-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/hybrid-failover-bot-1.0.5-r1_aarch64_cortex-a53.apk
apk add --allow-untrusted /tmp/luci-app-hybrid-failover-1.0.5-r1.apk
```

Самособранные пакеты без подписи OpenWrt keyring требуют `--allow-untrusted` (как в [документации сборки](https://git.openwrt.org/?p=openwrt/openwrt.git;a=blob;f=config/Config-build.in;hb=openwrt-25.12)).

## Установка с хоста по SSH

```sh
./scripts/install-from-local-dist.sh 192.168.42.1  # dist/ipk/ и/или dist/apk/
./scripts/deploy-telegram-bot.sh 192.168.42.1      # только бот (разработка)
```

## Зависимости

| Компонент | Пакеты (opkg / apk) |
|-----------|---------------------|
| Core | `sing-box`, `curl`, `coreutils-base64` |
| Бот | `uci`, `procd`, `hybrid-failover-core` (для RPC) |
| LuCI | `luci-base`, `luci-compat`, `hybrid-failover-core` |

В режиме `full` установщик сам ставит недостающие пакеты через `opkg` или `apk`.

## Релизы GitHub

Тег `v*.*.*` запускает [workflow сборки](../.github/workflows/release.yml): в [Releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) публикуются и `.ipk` (24.x), и `.apk` (25.12+).
