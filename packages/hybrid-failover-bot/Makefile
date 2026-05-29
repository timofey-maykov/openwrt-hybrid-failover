#
# OpenWrt SDK package (optional). Prebuild binaries with scripts/build-packages.sh
# and place them in ./binaries/<ARCH>/hybrid-failover-bot before make.
#
include $(TOPDIR)/rules.mk

PKG_NAME:=hybrid-failover-bot
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_MAINTAINER:=Hybrid Failover
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

GO_ARCH:=$(ARCH)
ifeq ($(ARCH),aarch64)
  GO_ARCH:=arm64
endif
ifeq ($(ARCH),mipsel)
  GO_ARCH:=mipsle
endif

define Package/hybrid-failover-bot
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Hybrid Failover Telegram management bot
  DEPENDS:=+uci +procd +ca-bundle
endef

define Package/hybrid-failover-bot/description
  Telegram bot for Hybrid Failover on OpenWrt (hybrid-failover UCI and status).
endef

define Build/Compile
	# Binary is built outside SDK via scripts/build-packages.sh
endef

define Package/hybrid-failover-bot/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./binaries/$(ARCH)/hybrid-failover-bot $(1)/usr/bin/hybrid-failover-bot
	$(INSTALL_DIR) $(1)/etc/init.d $(1)/etc/config
	$(INSTALL_BIN) ../../bot/openwrt/etc/init.d/hybrid-failover-bot $(1)/etc/init.d/
	$(INSTALL_CONF) ../../bot/openwrt/etc/config/hybrid-failover-bot $(1)/etc/config/
	$(INSTALL_DATA) ../../bot/openwrt/etc/hybrid-failover-bot.json $(1)/etc/
endef

$(eval $(call BuildPackage,hybrid-failover-bot))
