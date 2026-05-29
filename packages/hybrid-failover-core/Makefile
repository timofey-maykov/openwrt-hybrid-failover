include $(TOPDIR)/rules.mk

PKG_NAME:=hybrid-failover-core
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/hybrid-failover-core
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Hybrid Failover Core (Go)
  DEPENDS:=+sing-box +ca-bundle +uci +procd
endef

define Package/hybrid-failover-core/install
	$(INSTALL_DIR) $(1)/usr/sbin $(1)/etc/init.d
	$(INSTALL_BIN) ../../dist/binaries/$(ARCH)/hybrid-failover $(1)/usr/sbin/hybrid-failover
	$(INSTALL_BIN) ../../openwrt/etc/init.d/hybrid-failover $(1)/etc/init.d/hybrid-failover
endef

$(eval $(call BuildPackage,hybrid-failover-core))
