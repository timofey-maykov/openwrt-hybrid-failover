# Файлы пакета podkop-hybrid-failover

Содержимое для сборки `.ipk` и установки с GitHub (в CI и на роутере).

| Файл | Назначение |
|------|------------|
| `usr/bin/podkop` | Патченный `/usr/bin/podkop` (hybrid failover, urltest) |

Остальные файлы пакета берутся из корня репозитория: `vendor/sing_box_config_facade.sh`, `scripts/amnezia_vpn_uri_to_vless.py`, `luci/section.js`.

Локальная копия с роутера по-прежнему может лежать в `work/` (не в git).
