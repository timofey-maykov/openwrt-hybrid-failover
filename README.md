# Podkop Hybrid Failover

Доработка **Podkop** для OpenWrt: **VPN (AmneziaWG / bind_interface) + резервные proxy** через **urltest**, поддержка **`vpn://`**, расширенный **URLTest**, **Telegram-бот** и **LuCI** (RU).

Репозиторий: [github.com/timofey-maykov/podkop-hybrid-failover](https://github.com/timofey-maykov/podkop-hybrid-failover)

**Полное описание (архитектура, URI, UCI, установка, бот, диагностика):** [**docs/OVERVIEW.md**](docs/OVERVIEW.md)

---

## Кратко

| Компонент | Что даёт |
|-----------|----------|
| **Hybrid failover** | VPN-интерфейс + резервные `vless`/`ss`/`trojan`/`socks`/`hy2`/`vpn://` через urltest |
| **Пакет `podkop-hybrid-failover`** | Патченный `/usr/bin/podkop`, facade, `section.js`, Amnezia-декодер |
| **Telegram-бот** | Управление с телефона: UCI, failover, `/health`, pending-конфиг |
| **LuCI** | Форма секции Podkop + страница «Telegram-бот Podkop» |

Базовая версия upstream: **podkop v0.7.10**.

---

## Установка на роутер

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/podkop-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Скрипт сам определяет **архитектуру**, качает **релиз** с GitHub и ставит пакеты (`opkg`).

| `PODKOP_HF_MODE` | Содержимое |
|------------------|------------|
| `full` (по умолчанию) | hybrid + бот + LuCI бота |
| `bot` | только бот + LuCI |
| `patches` | только hybrid |

Подробнее: [docs/INSTALL.md](docs/INSTALL.md). Сборка `.ipk`: `./scripts/build-packages.sh`.

После установки бота: токен из [@BotFather](https://t.me/BotFather) → `/etc/podkop-telegram-bot.json` → `uci set podkop-telegram-bot.main.enabled=1` → `/panel` в Telegram.

---

## Поддерживаемые ссылки (failover / urltest)

| Схема | Роутер (Podkop) | Telegram-бот |
|-------|-----------------|----------------|
| `vless://` | да | да |
| `ss://` | да | да |
| `trojan://` | да | да |
| `socks4/4a/5://` | да | нет* |
| `hysteria2://`, `hy2://` | да | нет* |
| `vpn://` (Amnezia) | да | да |
| `awg2://` | да (служебный URI)* | нет* |

\*`awg2://` это **не** отдельный сетевой протокол. В Podkop так помечается внутренняя ссылка для настройки интерфейса **AmneziaWG 2.0** (создание `amneziawg`, `awg setconf`, direct outbound). Обычно появляется при конвертации `vpn://`, если в контейнере Amnezia указан `amnezia-awg2`. Подробнее: [docs/OVERVIEW.md](docs/OVERVIEW.md#amnezia-awg2-awg2).

\*В боте при добавлении URI проверяются только `vless`, `trojan`, `ss`, `vpn`: остальное добавляйте через UCI/LuCI.

Два режима: **VPN + failover** (`failover_proxy_links`) и **proxy URLTest** (`urltest_proxy_links`). Схема outbounds: [docs/OVERVIEW.md#архитектура](docs/OVERVIEW.md#архитектура).

---

## Telegram-бот и LuCI

| | |
|---|---|
| Документация бота | [bot/README.md](bot/README.md) |
| LuCI бота | **Сервисы → Telegram-бот Podkop** |
| LuCI Podkop (секция) | поля failover в `section.js` |

---

## Документация

| Файл | Тема |
|------|------|
| [**docs/OVERVIEW.md**](docs/OVERVIEW.md) | **Полное описание проекта** |
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
| `packaging/` | Файлы для сборки `podkop-hybrid-failover` |
| `patches/` | Патчи к podkop, facade, LuCI `main.js` |
| `luci/` | `section.js`, fe-app |
| `bot/` | Telegram-бот (Go) |
| `scripts/` | `install-on-router.sh`, `build-packages.sh`, Amnezia-декодер |
| `docs/` | Документация |

Upstream-патчи: `patches/upstream-main/README.md`.
