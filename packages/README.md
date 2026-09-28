[English](README.en.md)

# Пакеты OpenWrt

Пакеты собираются на ПК или в CI без OpenWrt SDK. Нужны Go и tar, для `.apk` ещё apk-tools 3.x или Docker.

```sh
chmod +x scripts/build-packages.sh scripts/lib/*.sh
./scripts/build-packages.sh
```

Скрипт кросс-компилирует `hybrid-failover` (core) и `hybrid-failover-bot` под каждую архитектуру, раскладывает файлы по пакетам и пишет `dist/manifest.json` с sha256 и размером каждого пакета. Этот манифест потом использует `hybrid-failover update`.

## Переменные сборки

| Переменная | По умолчанию | Что делает |
|------------|--------------|------------|
| `HF_PKG_FORMAT` | `both` | `ipk`, `apk` или `both` |
| `HF_UPX` | `auto` | `auto` сжимает бинарники UPX, если `upx` есть в PATH. `1` требует UPX, `0` отключает |
| `HF_BUILD_SET` | `full` | `full` собирает всё, `core` только core и бота без LuCI |
| `HF_BUILD_ARCHS_OVERRIDE` | | список архитектур через пробел, например `"x86_64"` |
| `PKG_RELEASE` | `1` | номер ревизии пакета (`-1` в ipk, `-r1` в apk) |
| `DIST_DIR` | `dist` | куда складывать результат |
| `APK_DOCKER_IMAGE` | `alpine:edge` | образ для `apk mkpkg`, если на хосте нет подходящего `apk` |

Версия берётся из файла `VERSION` в корне репозитория. В CI (`.github/workflows/release.yml`) сборка идёт с `HF_UPX=1`, UPX уменьшает бинарники примерно на 60%.

## Результат

| Каталог | Формат | OpenWrt |
|---------|--------|---------|
| `dist/ipk/` | `.ipk` (opkg) | 24.10 и старше |
| `dist/apk/` | `.apk` (apk-tools 3) | 25.12+ |
| `dist/binaries/<arch>/` | голые бинарники | |

Поддерживаются архитектуры `aarch64_cortex-a53`, `aarch64_generic`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc` и `x86_64`.

### Имена файлов

| opkg (24.x) | apk (25.12+) |
|-------------|--------------|
| `hybrid-failover-core_1.7.48-1_aarch64_cortex-a53.ipk` | `hybrid-failover-core-1.7.48-r1_aarch64_cortex-a53.apk` |
| `luci-app-hybrid-failover_1.7.48-1_all.ipk` | `luci-app-hybrid-failover-1.7.48-r1.apk` |

Версия в apk следует [схеме OpenWrt](https://git.openwrt.org/?p=openwrt/openwrt.git;a=commit;h=e8725a932e16eaf6ec51add8c084d959cbe32ff2), ревизия пишется как `-rN`. У пакетов без архитектуры (`all` в ipk, `noarch` в apk) суффикса архитектуры в имени apk нет.

## Состав

| Пакет | Архитектура | Содержимое |
|-------|-------------|------------|
| `hybrid-failover-core` | по цели | `/usr/sbin/hybrid-failover` со встроенным движком, init.d, `/etc/config/hybrid-failover`, `/etc/sysctl.d/20-hybrid-failover.conf` |
| `hybrid-failover-bot` | по цели | `/usr/bin/hybrid-failover-bot`, init.d, UCI `hybrid-failover-bot`, `/etc/hybrid-failover-bot.json` |
| `luci-app-hybrid-failover` | all | страницы LuCI: обзор, маршрутизация, клиенты, диагностика, Telegram, обновление |
| `luci-i18n-hybrid-failover` | all | переводы LuCI |
| `curfew` | all | ночные ограничения скорости для выбранных устройств LAN (tc) |
| `luci-app-curfew` | all | LuCI для curfew |

Внешний sing-box не нужен, движок встроен в core. `kmod-amneziawg` и `amneziawg-tools` в сборку не входят. Про AmneziaWG 3.1 написано в [docs/INSTALL.md](../docs/INSTALL.md#amneziawg-31).

При обновлении пакета сохраняются `/etc/config/hybrid-failover`, `/etc/config/hybrid-failover-bot` и `/etc/hybrid-failover-bot.json`, они объявлены как conffiles. Postinst core создаёт `/etc/hybrid-failover/pending`, запускает `hybrid-failover migrate` и применяет sysctl. Postinst бота включает его init-скрипт, но сам бот запускается только при `hybrid-failover-bot.main.enabled=1`.

## Установка на роутере

`scripts/install-on-router.sh` выбирает `apk` или `opkg` и скачивает подходящие файлы из GitHub Releases.

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

По умолчанию (`HF_MODE=full`) ставятся `hybrid-failover-core`, `luci-i18n-hybrid-failover`, `luci-app-hybrid-failover` и `hybrid-failover-bot`, причём переводы идут раньше LuCI-приложения, которое от них зависит. `HF_MODE=core` ставит то же без бота, `HF_MODE=bot` ставит только бота на роутер, где core уже есть.

Свежесобранные пакеты можно поставить с ПК командой `./scripts/install-from-local-dist.sh <ip роутера>`. Подробности в [docs/INSTALL.md](../docs/INSTALL.md).

## OpenWrt SDK

Makefile в `hybrid-failover-core/`, `hybrid-failover-bot/` и `luci-app-hybrid-failover-bot/` оставлены для сборки в дереве OpenWrt, но они отстают от `build-packages.sh`. Версия в них зашита как `1.0.0`, Makefile core ставит только бинарник и init-скрипт, а `luci-app-hybrid-failover-bot` описывает старую отдельную страницу бота, которой нет в релизах. Бинарники в любом случае собираются снаружи. Сначала выполните `scripts/build-packages.sh`, затем положите бота в `packages/hybrid-failover-bot/binaries/<ARCH>/`. Makefile core читает бинарник из `dist/binaries/<ARCH>/`.
