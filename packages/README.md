# OpenWrt packages

Готовые пакеты собираются на ПК/в CI без полного OpenWrt SDK:

```sh
chmod +x scripts/build-packages.sh scripts/lib/*.sh
./scripts/build-packages.sh
```

По умолчанию (`HF_PKG_FORMAT=both`):

| Каталог | Формат | OpenWrt |
|---------|--------|---------|
| `dist/ipk/` | `.ipk` (opkg) | 24.10 и ранее |
| `dist/apk/` | `.apk` (Alpine Package Keeper) | **25.12+** |

Только один формат: `HF_PKG_FORMAT=ipk` или `HF_PKG_FORMAT=apk`.

Сборка `.apk` использует `apk mkpkg` (apk-tools 3.x). Если на хосте нет подходящего `apk`, скрипт вызывает Docker (`alpine:edge`, переменная `APK_DOCKER_IMAGE`).

### Имена файлов

| opkg (24.x) | apk (25.12+) |
|-------------|----------------|
| `hybrid-failover-bot_1.0.5-1_aarch64_cortex-a53.ipk` | `hybrid-failover-bot-1.0.5-r1_aarch64_cortex-a53.apk` |
| `luci-app-hybrid-failover-bot_1.0.5-1_all.ipk` | `luci-app-hybrid-failover-bot-1.0.5-r1.apk` |

Версия APK следует [схеме OpenWrt](https://git.openwrt.org/?p=openwrt/openwrt.git;a=commit;h=e8725a932e16eaf6ec51add8c084d959cbe32ff2): релиз через `-rN` (например `1.0.5-r1`), не `-1`.

| Пакет | Architecture | Содержимое |
|-------|----------------|------------|
| `hybrid-failover-core` | per-target | Go core: `/usr/sbin/hybrid-failover`, init.d |
| `hybrid-failover-bot` | per-target | Telegram bot (uses core RPC) |
| `luci-app-hybrid-failover` | all | Unified LuCI: routing, dashboard, clients, bot |
| `luci-app-hybrid-failover-bot` | all | Legacy bot-only LuCI page |

Поддерживаемые GO/OpenWrt arch: `aarch64_cortex-a53`, `aarch64_generic`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc`, `x86_64`.

## Установка на роутере

`scripts/install-on-router.sh` определяет менеджер пакетов: **`apk`** (25.12+) или **`opkg`** (24.x) и качает подходящий артефакт из GitHub Releases.

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/openwrt-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Подробнее: [docs/INSTALL.md](../docs/INSTALL.md).

## OpenWrt SDK (опционально)

Makefiles в подкаталогах: для сборки в дереве OpenWrt. Сначала выполните `scripts/build-packages.sh` и скопируйте бинарники в `packages/hybrid-failover-bot/binaries/<ARCH>/`.
