# OpenWrt packages

Готовые `.ipk` собираются на ПК/в CI без полного OpenWrt SDK:

```sh
chmod +x scripts/build-packages.sh scripts/lib/*.sh
./scripts/build-packages.sh
```

Результат: `dist/ipk/`

| Пакет | Architecture | Содержимое |
|-------|----------------|------------|
| `podkop-telegram-bot` | per-target | Go-бинарник, init.d, UCI, JSON-конфиг |
| `luci-app-podkop-bot` | all | LuCI-страница бота (RU) |
| `podkop-hybrid-failover` | all | Патчи `/usr/bin/podkop`, facade, `section.js` |

Поддерживаемые GO/OpenWrt arch: `aarch64_cortex-a53`, `arm_cortex-a7`, `mipsel_24kc`, `mips_24kc`, `x86_64`.

## Установка на роутере

```sh
wget -O /tmp/install.sh \
  https://raw.githubusercontent.com/timofey-maykov/podkop-hybrid-failover/main/scripts/install-on-router.sh
ash /tmp/install.sh
```

Подробнее об установке и настройке бота: [docs/INSTALL.md](../docs/INSTALL.md).

## OpenWrt SDK (опционально)

Makefiles в подкаталогах: для сборки в дереве OpenWrt. Сначала выполните `scripts/build-packages.sh` и скопируйте бинарники в `packages/podkop-telegram-bot/binaries/<ARCH>/`.
