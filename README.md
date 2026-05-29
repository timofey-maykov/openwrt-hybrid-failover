# Hybrid Failover

Автономный стек маршрутизации для OpenWrt: **VPN (AmneziaWG / bind_interface) + резервные proxy** через **urltest**, поддержка **`vpn://`**, расширенный **URLTest**, **Telegram-бот** и **LuCI** (RU). Пакет **`hybrid-failover-core`**: без зависимости от `jq` и `python3-light`.

Релизы (`.ipk` / `.apk`): [github.com/timofey-maykov/openwrt-hybrid-failover/releases](https://github.com/timofey-maykov/openwrt-hybrid-failover/releases)

**Полное описание (архитектура, URI, UCI, установка, бот, диагностика):** [**docs/OVERVIEW.md**](docs/OVERVIEW.md)

---

## Кратко

| Компонент | Что даёт |
|-----------|----------|
| **`hybrid-failover-core`** | Go-бинарник `/usr/sbin/hybrid-failover`: UCI → sing-box, nft, dnsmasq, списки |
| **Hybrid failover** | VPN-интерфейс + резервные `vless`/`ss`/`trojan`/`socks`/`hy2`/`vpn://` через urltest |
| **Telegram-бот** | Управление с телефона: UCI `hybrid-failover`, failover, `/health`, pending-конфиг |
| **LuCI** | Маршрутизация, дашборд, per-client, Telegram-бот: **Сервисы → Hybrid Failover** |

Конфиг UCI: **`/etc/config/hybrid-failover`**. Первичная настройка и миграция: `hybrid-failover migrate`.

---

## Установка на роутер

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Скрипт определяет **архитектуру**, качает **релиз** с GitHub и ставит пакеты (`opkg` / `apk`).

| `HF_MODE` | Содержимое |
|-----------|------------|
| `full` (по умолчанию) | hybrid-failover-core + hybrid-failover-bot + luci-app-hybrid-failover |
| `bot` | только бот + LuCI |
| `core` | только hybrid-failover-core |

Подробнее: [docs/INSTALL.md](docs/INSTALL.md). Сборка `.ipk`: `./scripts/build-packages.sh`.

После установки бота: токен из [@BotFather](https://t.me/BotFather) → `/etc/hybrid-failover-bot.json` → `uci set hybrid-failover-bot.main.enabled=1` → `/panel` в Telegram.

---

## Поддерживаемые ссылки (failover / urltest)

| Схема | Core | Telegram-бот |
|-------|------|----------------|
| `vless://` | да | да |
| `ss://` | да | да |
| `trojan://` | да | да |
| `socks4/4a/5://` | да | нет* |
| `hysteria2://`, `hy2://` | да | нет* |
| `vpn://` (Amnezia) | да (Go-декодер) | да |
| `awg2://` | да (служебный URI)* | нет* |

\*`awg2://`: внутренняя ссылка для настройки **AmneziaWG 2.0** (direct outbound). Подробнее: [docs/OVERVIEW.md](docs/OVERVIEW.md).

\*В боте при добавлении URI проверяются `vless`, `trojan`, `ss`, `vpn`; остальное: через UCI/LuCI.

Два режима: **VPN + failover** (`failover_proxy_links`) и **proxy URLTest** (`urltest_proxy_links`).

---

## Telegram-бот и LuCI

| | |
|---|---|
| Документация бота | [bot/README.md](bot/README.md) |
| LuCI | **Сервисы → Hybrid Failover** |

---

## Документация

| Файл | Тема |
|------|------|
| [**docs/OVERVIEW.md**](docs/OVERVIEW.md) | **Полное описание проекта** |
| [docs/UCI.md](docs/UCI.md) | Опции UCI |
| [docs/INSTALL.md](docs/INSTALL.md) | Установка, режимы, зависимости |
| [packages/README.md](packages/README.md) | Пакеты `.ipk` / `.apk` |
| [luci/README.md](luci/README.md) | LuCI, дашборд, `vpn://` |
| [examples/glob-uci-commands.txt](examples/glob-uci-commands.txt) | Пример UCI для `glob` |

---

## Содержимое репозитория

| Путь | Назначение |
|------|------------|
| `core/`, `internal/` | Go core |
| `bot/` | Telegram-бот |
| `luci/` | luci-app-hybrid-failover |
| `packages/` | Сборка `.ipk` / `.apk` |
| `openwrt/` | init.d, UCI-шаблон |
| `scripts/` | install-on-router.sh, build-packages.sh, QEMU lab |
| `docs/` | Документация |
| `legacy/` | Устаревшие patch-скрипты (не в релизе по умолчанию) |
