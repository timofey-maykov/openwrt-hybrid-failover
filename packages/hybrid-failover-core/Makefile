include $(TOPDIR)/rules.mk

PKG_NAME:=hybrid-failover-core
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/hybrid-failover-core
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Hybrid Failover Core (Go)
  DEPENDS:=+ca-bundle +uci +procd
endef

define Package/hybrid-failover-core/install
	$(INSTALL_DIR) $(1)/usr/sbin $(1)/etc/init.d $(1)/etc/hotplug.d/net
	$(INSTALL_BIN) ../../dist/binaries/$(ARCH)/hybrid-failover $(1)/usr/sbin/hybrid-failover
	$(INSTALL_BIN) ../../openwrt/etc/init.d/hybrid-failover $(1)/etc/init.d/hybrid-failover
	$(INSTALL_BIN) ../../openwrt/etc/hotplug.d/net/30-hybrid-failover-docker $(1)/etc/hotplug.d/net/30-hybrid-failover-docker
endef

$(eval $(call BuildPackage,hybrid-failover-core))
