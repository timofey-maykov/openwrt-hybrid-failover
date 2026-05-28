# Установка на OpenWrt

## Одна команда на роутере

После публикации [релиза](https://github.com/timofey-maykov/podkop-hybrid-failover/releases) на GitHub:

```sh
opkg update
opkg install curl ca-bundle wget

wget -O /tmp/podkop-install.sh \
  https://raw.githubusercontent.com/timofey-maykov/podkop-hybrid-failover/main/scripts/install-on-router.sh

ash /tmp/podkop-install.sh
```

По умолчанию ставится **полный набор**: патчи Podkop hybrid, Telegram-бот и LuCI-страница бота.

### Режимы установки

| `PODKOP_HF_MODE` | Что устанавливается |
|------------------|---------------------|
| `full` (по умолчанию) | Патчи Podkop + Telegram-бот + LuCI |
| `bot` | Только `podkop-telegram-bot` и `luci-app-podkop-bot` |
| `patches` | Только `podkop-hybrid-failover` (без бота) |

### Переменные окружения

```sh
PODKOP_HF_REPO=timofey-maykov/podkop-hybrid-failover   # репозиторий GitHub
PODKOP_HF_VERSION=latest                               # или v1.0.1, v1.0.2, …
PODKOP_HF_BRANCH=main                                  # ветка для fallback-файлов
PODKOP_HF_TOKEN=123456789:ABC...                       # опционально: токен в JSON на роутере
PODKOP_HF_ADMIN_IDS=123456789                          # ваш Telegram user ID (подсказка в логе)
```

### После установки

1. Отредактируйте `/etc/podkop-telegram-bot.json`: `token`, `admin_ids`, `clash_api` (часто `http://192.168.x.1:9090`, не `127.0.0.1`).
2. Включите сервис бота:
   ```sh
   uci set podkop-telegram-bot.main.enabled=1
   uci commit podkop-telegram-bot
   /etc/init.d/podkop-telegram-bot restart
   ```
3. Откройте LuCI: **Сервисы → Telegram-бот Podkop**  
   `http://ROUTER/cgi-bin/luci/admin/services/podkop-bot`
4. В Telegram откройте своего бота и отправьте `/panel`.

## Ручная установка .ipk

На ПК соберите пакеты:

```sh
./scripts/build-packages.sh
```

Скопируйте `dist/ipk/` на роутер и установите (подставьте свою архитектуру):

```sh
. /etc/openwrt_release && echo "$DISTRIB_ARCH"

opkg install /tmp/podkop-telegram-bot_*_aarch64_cortex-a53.ipk
opkg install /tmp/luci-app-podkop-bot_*_all.ipk
opkg install /tmp/podkop-hybrid-failover_*_all.ipk
```

## Установка с хоста по SSH

```sh
./scripts/patch-router-all.sh 192.168.42.1      # патчи Podkop + LuCI section.js
./scripts/deploy-telegram-bot.sh 192.168.42.1 # только бот (разработка)
./scripts/install-from-local-dist.sh 192.168.42.1  # локальные .ipk из dist/ipk/
```

## Зависимости

| Компонент | Пакеты opkg |
|-----------|-------------|
| Бот | `uci`, `procd` (обычно уже в системе) |
| Hybrid failover | `podkop`, `sing-box`, `jq`, `curl`, `python3-light`, `coreutils-base64` |
| LuCI-страница бота | `luci-base`, `luci-compat` |

В режимах `full` и `patches` установщик сам ставит недостающие пакеты через `opkg`.

## Релизы GitHub

Тег `v*.*.*` запускает [workflow сборки](../.github/workflows/release.yml): все `.ipk` публикуются в [Releases](https://github.com/timofey-maykov/podkop-hybrid-failover/releases).
