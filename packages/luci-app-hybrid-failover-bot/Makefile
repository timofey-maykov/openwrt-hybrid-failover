#
# OpenWrt SDK: install LuCI files from ../../luci/root via scripts/build-packages.sh
# or copy manually. See packages/README.md
#
include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-hybrid-failover-bot
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-hybrid-failover-bot
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=Hybrid Failover Telegram Bot (LuCI)
  DEPENDS:=+luci-base +luci-compat +hybrid-failover-bot
endef

define Package/luci-app-hybrid-failover-bot/install
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/hybrid-failover
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ../../luci/root/www/luci-static/resources/view/hybrid-failover/bot.js \
		$(1)/www/luci-static/resources/view/hybrid-failover/
	$(INSTALL_DATA) ../../luci/root/usr/share/luci/menu.d/luci-app-hybrid-failover-bot.json \
		$(1)/usr/share/luci/menu.d/
	$(INSTALL_DATA) ../../luci/root/usr/share/rpcd/acl.d/luci-app-hybrid-failover-bot.json \
		$(1)/usr/share/rpcd/acl.d/
endef

$(eval $(call BuildPackage,luci-app-hybrid-failover-bot))
