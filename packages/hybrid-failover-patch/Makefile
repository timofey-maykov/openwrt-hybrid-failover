include $(TOPDIR)/rules.mk

PKG_NAME:=hybrid-failover-patch
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

include $(INCLUDE_DIR)/package.mk

define Package/hybrid-failover-patch
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Hybrid Failover patches for upstream podkop
  DEPENDS:=+podkop +sing-box +jq +curl +python3-light +coreutils-base64
endef

define Package/hybrid-failover-patch/install
	$(INSTALL_DIR) $(1)/usr/bin $(1)/usr/lib/podkop
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/podkop
	$(INSTALL_BIN) ../../packaging/hybrid-failover-patch/usr/bin/podkop $(1)/usr/bin/podkop
	$(INSTALL_DATA) ../../vendor/sing_box_config_facade.sh $(1)/usr/lib/podkop/
	$(INSTALL_DATA) ../../scripts/amnezia_vpn_uri_to_vless.py $(1)/usr/lib/podkop/
	$(INSTALL_DATA) ../../luci/section.js $(1)/www/luci-static/resources/view/podkop/section.js
endef

$(eval $(call BuildPackage,hybrid-failover-patch))
