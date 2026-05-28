# Товарные знаки Podkop

Проект **Hybrid Failover** не является [Podkop](https://github.com/itdoginfo/podkop), не поддерживается itdoginfo и не имеет официального статуса.

Программное обеспечение в этом репозитории основано на программном обеспечении Podkop с открытым исходным кодом и предназначено для совместимости с пакетом `podkop` на OpenWrt.

Использование названия и знаков Podkop регулируется [руководством по товарным знакам itdoginfo/podkop](https://github.com/itdoginfo/podkop/blob/docs/trademark/TRADEMARK_RU.md). Кратко для нашей модифицированной версии:

| Разрешено | Запрещено |
|-----------|-----------|
| Фактически указать происхождение: «основано на Podkop с открытым исходным кодом» | Называть продукт Podkop, Podkop Pro/Lite, MyPodkop и т.п. |
| Описать совместимость с UCI `podkop`, пакетом `podkop`, `luci-app-podkop` | Выдавать модификацию за официальный Podkop или подразумевать одобрение itdoginfo |
| Упоминать пути upstream (`/usr/bin/podkop`, `/usr/lib/podkop/`) | Использовать логотипы и брендинг Podkop в UI, документации и названиях **наших** пакетов |

## Наши обозначения (отдельный брендинг)

| Было (нарушало политику) | Стало |
|--------------------------|--------|
| Podkop Hybrid Failover | **Hybrid Failover** |
| `podkop-hybrid-failover` (пакет) | `hybrid-failover-patch` |
| `podkop-telegram-bot` | `hybrid-failover-bot` |
| `luci-app-podkop-bot` | `luci-app-hybrid-failover-bot` |

Репозиторий GitHub: [timofey-maykov/openwrt-hybrid-failover](https://github.com/timofey-maykov/openwrt-hybrid-failover). Старый URL `podkop-hybrid-failover` перенаправляется автоматически.

## Что не переименовывали

- Пакет upstream **`podkop`** и конфиг UCI **`podkop.*`**: это компоненты оригинального проекта.
- Патченный бинарник остаётся **`/usr/bin/podkop`**: замена файла в составе патча для совместимости с `opkg`/`init.d` upstream.
- LuCI **`section.js`** ставится в путь **`view/podkop/`** формы upstream **luci-app-podkop**.

При вопросах по бренду Podkop обращайтесь к правообладателю через репозиторий [itdoginfo/podkop](https://github.com/itdoginfo/podkop).
