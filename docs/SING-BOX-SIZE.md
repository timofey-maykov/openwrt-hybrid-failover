# Размер пакетов и overlay

OpenWrt с overlay **64–128 MiB** быстро заполняется: stock **sing-box** ~40 MiB, наши бинарники ~4 MiB, LuCI и системные обновления.

## Что занимает место

| Компонент | Типичный размер | Примечание |
|-----------|-----------------|------------|
| sing-box (stock) | ~38–41 MiB | все теги (tailscale, wireguard, …) |
| sing-box-lite | ~15–25 MiB | `./scripts/build-sing-box-lite.sh` |
| hybrid-failover core | ~1.5–2 MiB | `/usr/sbin/hybrid-failover` |
| hybrid-failover-bot | ~1.5–2 MiB | опционально |
| bind-libs + dig | ~1.3 MiB | **не нужен**: `check-fakeip` через Go DNS |

## sing-box-lite

Сборка урезанного sing-box без tailscale/wireguard/dhcp:

```sh
./scripts/build-sing-box-lite.sh aarch64_cortex-a53
# HF_UPX=1 — дополнительное сжатие
```

На роутере (мало места — сначала снять старый пакет):

```sh
/etc/init.d/hybrid-failover stop
/etc/init.d/sing-box stop
opkg remove --force-depends sing-box
cp /tmp/sing-box /usr/bin/sing-box
chmod 755 /usr/bin/sing-box
/etc/init.d/sing-box start
/etc/init.d/hybrid-failover start
```

Проверка: `hybrid-failover status`, `sing-box version`.

## Наши бинарники (UPX)

CI и `./scripts/build-packages.sh` с **`HF_UPX=1`** сжимают core/bot через UPX (~40–60% меньше). На роутере старт на доли секунды дольше.

## Установка при нехватке места

`scripts/install-on-router.sh`:

- проверяет свободное место на `/overlay`;
- при нехватке снимает старый пакет перед установкой (`remove → install`);
- использует `opkg install --force-space`.

Переменные:

| Переменная | Значение |
|------------|----------|
| `HF_LOW_SPACE_KB` | порог «мало места» (по умолчанию 45000) |
| `HF_FORCE_REINSTALL` | `1` — всегда remove→install для наших пакетов |

## Legacy `/usr/bin/hybrid-failover`

Старые установки оставляли монолитный бинарник **6+ MiB** в `/usr/bin`. Postinst core удаляет его, если есть `/usr/sbin/hybrid-failover`.

## Рекомендации

1. Роутер с overlay **128+ MiB** или extroot.
2. sing-box-lite на устройствах **64 MiB**.
3. Без Telegram — не ставить `hybrid-failover-bot`.
4. `bind-dig` можно снять: `opkg remove bind-dig bind-libs`.
