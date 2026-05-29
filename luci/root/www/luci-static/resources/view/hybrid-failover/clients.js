'use strict';
'require view';
'require form';
'require uci';
'require rpc';

var callReload = rpc.declare({
	object: 'hybrid-failover',
	method: 'reload'
});

return view.extend({
	load: function() {
		return uci.load('hybrid-failover');
	},

	render: function() {
		var m = new form.Map('hybrid-failover', _('Hybrid Failover: клиенты'),
			_('Кто ходит через Hybrid Failover: include/exclude по source IP клиента LAN (DHCP). Это не подсети назначения: их задают в «Маршрутизация» (user_subnets / community lists).'));
		this.map = m;

		var s = m.section(form.NamedSection, 'settings', 'settings', _('Per-client rules'));
		s.option(form.DynamicList, 'include_source_ips', _('Include source IPs (через Hybrid Failover)'));
		s.option(form.DynamicList, 'exclude_source_ips', _('Exclude source IPs (миновать Hybrid Failover)'));

		return m.render();
	},

	handleSaveApply: function() {
		return this.map.save(true).then(function() {
			return callReload();
		});
	},

	handleSave: function() {
		return this.map.save(false).then(function() {
			return callReload();
		});
	}
});
