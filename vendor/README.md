# Vendor snapshot

- **`podkop.from-router.sh`**: копия `/usr/bin/podkop` с вашего OpenWrt (после правки post-hook failover). Используется как основа для `diff` и проверки патча локально.

Для чистого сравнения со stock снимите файл с роутера **до** любых ручных вставок в `/usr/bin/podkop` или восстановите из `opkg`.
