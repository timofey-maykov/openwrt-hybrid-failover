# Hybrid Failover

> **Не является Podkop** и не одобрено itdoginfo. Проект основан на [Podkop](https://github.com/itdoginfo/podkop) с открытым исходным кодом. См. [docs/TRADEMARK.md](docs/TRADEMARK.md) и [политику товарных знаков Podkop](https://github.com/itdoginfo/podkop/blob/docs/trademark/TRADEMARK_RU.md).

Дополнения для OpenWrt: **VPN (AmneziaWG / bind_interface) + резервные proxy** через **urltest**, поддержка **`vpn://`**, расширенный **URLTest**, **Telegram-бот** и **LuCI** (RU). Требуется установленный upstream-пакет **podkop**.

Репозиторий: [github.com/timofey-maykov/openwrt-hybrid-failover](https://github.com/timofey-maykov/openwrt-hybrid-failover)

**Полное описание (архитектура, URI, UCI, установка, бот, диагностика):** [**docs/OVERVIEW.md**](docs/OVERVIEW.md)

---

## Кратко

| Компонент | Что даёт |
|-----------|----------|
| **Hybrid failover** | VPN-интерфейс + резервные `vless`/`ss`/`trojan`/`socks`/`hy2`/`vpn://` через urltest |
| **Пакет `hybrid-failover-patch`** | Патченный `/usr/bin/podkop`, facade, `section.js`, Amnezia-декодер |
| **Telegram-бот** | Управление с телефона: UCI `podkop`, failover, `/health`, pending-конфиг |
| **LuCI** | Поля failover в форме upstream **luci-app-podkop** + страница **Telegram-бот Hybrid Failover** |

Базовая версия upstream: **podkop v0.7.10**.

---

## Установка на роутер

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Скрипт сам определяет **архитектуру**, качает **релиз** с GitHub и ставит пакеты (`opkg`).

| `HF_MODE` | Содержимое |
|------------------|------------|
| `full` (по умолчанию) | hybrid-failover-patch + hybrid-failover-bot + luci-app-hybrid-failover-bot |
| `bot` | только бот + LuCI |
| `patches` | только hybrid-failover-patch |

Переменные `PODKOP_HF_*` по-прежнему принимаются (устаревшие алиасы к `HF_*`).

Подробнее: [docs/INSTALL.md](docs/INSTALL.md). Сборка `.ipk`: `./scripts/build-packages.sh`.

После установки бота: токен из [@BotFather](https://t.me/BotFather) → `/etc/hybrid-failover-bot.json` → `uci set hybrid-failover-bot.main.enabled=1` → `/panel` в Telegram.

---

## Поддерживаемые ссылки (failover / urltest)

| Схема | Роутер (patched podkop) | Telegram-бот |
|-------|-------------------------|----------------|
| `vless://` | да | да |
| `ss://` | да | да |
| `trojan://` | да | да |
| `socks4/4a/5://` | да | нет* |
| `hysteria2://`, `hy2://` | да | нет* |
| `vpn://` (Amnezia) | да | да |
| `awg2://` | да (служебный URI)* | нет* |

\*`awg2://` это **не** отдельный сетевой протокол. В патче podkop так помечается внутренняя ссылка для настройки интерфейса **AmneziaWG 2.0** (создание `amneziawg`, `awg setconf`, direct outbound). Обычно появляется при конвертации `vpn://`, если в контейнере Amnezia указан `amnezia-awg2`. Подробнее: [docs/OVERVIEW.md](docs/OVERVIEW.md#amnezia-awg2-awg2).

\*В боте при добавлении URI проверяются только `vless`, `trojan`, `ss`, `vpn`: остальное добавляйте через UCI/LuCI.

Два режима: **VPN + failover** (`failover_proxy_links`) и **proxy URLTest** (`urltest_proxy_links`). Схема outbounds: [docs/OVERVIEW.md#архитектура](docs/OVERVIEW.md#архитектура).

---

## Telegram-бот и LuCI

| | |
|---|---|
| Документация бота | [bot/README.md](bot/README.md) |
| LuCI бота | **Сервисы → Telegram-бот Hybrid Failover** |
| LuCI upstream | секция маршрутизации **luci-app-podkop** (`section.js`) |

---

## Документация

| Файл | Тема |
|------|------|
| [**docs/OVERVIEW.md**](docs/OVERVIEW.md) | **Полное описание проекта** |
| [docs/TRADEMARK.md](docs/TRADEMARK.md) | Товарные знаки Podkop |
| [docs/UCI.md](docs/UCI.md) | Опции UCI |
| [docs/INSTALL.md](docs/INSTALL.md) | Установка, режимы, зависимости |
| [packages/README.md](packages/README.md) | Пакеты `.ipk` |
| [luci/README.md](luci/README.md) | LuCI, дашборд, `vpn://` |
| [examples/glob-uci-commands.txt](examples/glob-uci-commands.txt) | Пример UCI для `glob` |

---

## Ручной патч (без пакетов)

```sh
./scripts/patch-router-all.sh ROUTER_IP
```

Или патч `patches/podkop-0.7.10-hybrid-urltest.patch` на роутере: см. [docs/OVERVIEW.md](docs/OVERVIEW.md).

**Важно:** уберите старый пост-хук `podkop-failover-apply.sh`, если был: иначе двойной failover.

---

## Содержимое репозитория

| Путь | Назначение |
|------|------------|
| `packaging/` | Файлы для сборки `hybrid-failover-patch` |
| `patches/` | Патчи к podkop, facade, LuCI `main.js` |
| `luci/` | `section.js`, fe-app |
| `bot/` | Telegram-бот (Go) |
| `scripts/` | `install-on-router.sh`, `build-packages.sh`, Amnezia-декодер |
| `docs/` | Документация |

Upstream-патчи: `patches/upstream-main/README.md`.
