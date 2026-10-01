# Hybrid Failover

[English](README.en.md) | Русский

Маршрутизация для OpenWrt с автоматическим резервом. Основной канал идёт через VPN (AmneziaWG). Если он падает, трафик уходит на резервные proxy, а после восстановления возвращается обратно. Устройствам в сети ничего настраивать не нужно.

Всё работает внутри одного Go-бинарника на роутере. Внешний sing-box, jq и python не нужны. Управлять можно из LuCI и из Telegram.

![Как работает Hybrid Failover](docs/img/hybrid-failover-schema.png)

## Что умеет

- Прозрачно перехватывает трафик LAN через nft tproxy и направляет его по правилам. Правила задаются доменами, подсетями и community-списками (например, youtube, telegram, russia_inside).
- DNS отвечает fakeip-адресами из 198.18.0.0/15. Поэтому трафик к нужным доменам попадает в движок ещё до реального резолва.
- Каналы проверяются через urltest. Живой и самый быстрый выбирается автоматически, есть политики `outage-only`, `prefer-primary` и `fastest`.
- Поддерживает ссылки `vless://`, `ss://`, `trojan://`, `hysteria2://`, `socks5://` и экспорт Amnezia `vpn://`, включая AmneziaWG 3.1.
- Правила для отдельных устройств по IP. Например, консоль всегда через VPN, а телевизор мимо.
- Списки сервисов можно развести по разным туннелям, чтобы YouTube не делил канал со всем остальным. Для каждого списка выбирается канал и запасной путь на случай, если канал упал, есть автораспределение и балансировка. Свои списки доменов и подсетей привязываются так же.
- Живые графики по каждому туннелю в LuCI: трафик, соединения, задержка и доля трафика, от 5 минут до суток.
- Telegram-бот показывает статус и каналы, переключает их, редактирует конфиг и присылает уведомления о переключениях.

![Графики каналов в LuCI](docs/img/luci-charts.png)

## Установка

На роутере с OpenWrt 24.x или 25.12:

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Скрипт сам определит архитектуру, скачает последний [релиз](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases) и поставит пакеты через opkg или apk.

| `HF_MODE` | Что ставится |
|-----------|--------------|
| `full` (по умолчанию) | сервис, LuCI и Telegram-бот |
| `core` | сервис и LuCI, без бота |
| `bot` | только Telegram-бот |

Режим задаётся переменной окружения, например `HF_MODE=core ash /tmp/install.sh`. Вкладка Telegram в LuCI появляется, только если бот установлен.

После установки откройте в LuCI раздел Сервисы, затем Hybrid Failover. Обновление до новой версии делается из LuCI или командой `hybrid-failover update apply`.

Для AmneziaWG нужны пакеты `kmod-amneziawg` и `amneziawg-tools` под вашу версию OpenWrt. В релиз они не входят. Подробности в [docs/INSTALL.md](docs/INSTALL.md).

## Поддерживаемые ссылки

| Схема | Примечание |
|-------|------------|
| `vless://` | Reality, XTLS, транспорт из параметров |
| `ss://` | Shadowsocks |
| `trojan://` | |
| `socks4://`, `socks4a://`, `socks5://` | UDP over TCP через `enable_udp_over_tcp` |
| `hysteria2://`, `hy2://` | TLS, obfs, ограничения скорости |
| `vpn://` | Экспорт Amnezia. Превращается в `vless://` или `awg2://` |
| `awg2://` | Служебная ссылка core для AmneziaWG, в том числе 3.1 |

Ссылки добавляются из LuCI или из Telegram-бота. Проверяет их core при применении.

## Telegram-бот

Бот работает на роутере и управляет им через core. Токен берётся у [@BotFather](https://t.me/BotFather) и прописывается в `/etc/hybrid-failover-bot.json` вместе с `admin_ids`. Потом включите сервис:

```sh
uci set hybrid-failover-bot.main.enabled=1 && uci commit hybrid-failover-bot
/etc/init.d/hybrid-failover-bot restart
```

В Telegram откройте `/panel`. Если роутеров несколько, дайте каждому своего бота или подключите их к одному боту по SSH. Обе схемы описаны в [bot/README.md](bot/README.md).

## Документация

| Файл | О чём |
|------|-------|
| [docs/OVERVIEW.md](docs/OVERVIEW.md) | Архитектура, режимы маршрутизации, DNS, failover, правила для устройств |
| [docs/INSTALL.md](docs/INSTALL.md) | Установка, обновление, AmneziaWG 3.1 |
| [docs/LUCI.md](docs/LUCI.md) | Веб-интерфейс по вкладкам |
| [docs/UCI.md](docs/UCI.md) | Все опции `/etc/config/hybrid-failover` |
| [bot/README.md](bot/README.md) | Telegram-бот, команды, несколько роутеров |
| [luci/README.md](luci/README.md) | Исходники LuCI и rpcd |
| [packages/README.md](packages/README.md) | Сборка пакетов `.ipk` и `.apk` |

## Структура репозитория

| Путь | Что там |
|------|---------|
| `core/`, `internal/` | Go core и движок |
| `bot/` | Telegram-бот |
| `luci/` | luci-app-hybrid-failover |
| `openwrt/` | init.d и шаблон UCI |
| `packages/` | Сборка пакетов |
| `scripts/` | Установка, сборка, тестовые стенды на QEMU |
| `docs/` | Документация на русском, `docs/en/` на английском |
| `legacy/` | Старые скрипты, в релиз не входят |

Сборка пакетов локально выполняется командой `./scripts/build-packages.sh`. Релиз собирает GitHub Actions при пуше тега `v*`.
