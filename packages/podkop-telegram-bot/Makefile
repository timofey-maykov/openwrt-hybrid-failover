#
# OpenWrt SDK package (optional). Prebuild binaries with scripts/build-packages.sh
# and place them in ./binaries/<ARCH>/podkop-telegram-bot before make.
#
include $(TOPDIR)/rules.mk

PKG_NAME:=podkop-telegram-bot
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_MAINTAINER:=Podkop HF
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

GO_ARCH:=$(ARCH)
ifeq ($(ARCH),aarch64)
  GO_ARCH:=arm64
endif
ifeq ($(ARCH),mipsel)
  GO_ARCH:=mipsle
endif

define Package/podkop-telegram-bot
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Podkop Telegram management bot
  DEPENDS:=+uci +procd +ca-bundle
endef

define Package/podkop-telegram-bot/description
  Lightweight Telegram bot to manage Podkop/failover on OpenWrt.
endef

define Build/Compile
	# Binary is built outside SDK via scripts/build-packages.sh
endef

define Package/podkop-telegram-bot/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./binaries/$(ARCH)/podkop-telegram-bot $(1)/usr/bin/podkop-telegram-bot
	$(INSTALL_DIR) $(1)/etc/init.d $(1)/etc/config
	$(INSTALL_BIN) ../../bot/openwrt/etc/init.d/podkop-telegram-bot $(1)/etc/init.d/
	$(INSTALL_CONF) ../../bot/openwrt/etc/config/podkop-telegram-bot $(1)/etc/config/
	$(INSTALL_DATA) ../../bot/openwrt/etc/podkop-telegram-bot.json $(1)/etc/
endef

$(eval $(call BuildPackage,podkop-telegram-bot))
