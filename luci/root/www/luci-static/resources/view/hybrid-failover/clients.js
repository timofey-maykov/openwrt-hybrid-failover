'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';

var callReload = rpc.declare({
	object: 'hybrid-failover',
	method: 'reload'
});

var callDhcpLeases = rpc.declare({
	object: 'hybrid-failover',
	method: 'dhcp_leases'
});

var MODES = [
	['include', _('Include: через Hybrid Failover (nft mark)')],
	['exclude', _('Exclude: миновать Hybrid Failover')],
	['full_route', _('Full route: весь трафик клиента через секцию')],
	['global_exclude', _('Global exclude: исключить из tproxy маршрутизации')]
];

return view.extend({
	load: function() {
		return uci.load('hybrid-failover');
	},

	render: function() {
		var m = new form.Map('hybrid-failover', _('Hybrid Failover: клиенты'),
			_('Единые правила per-client (client_rule). Legacy list-опции читаются, пока нет client_rule секций.'));
		this.map = m;

		var s = m.section(form.TypedSection, 'client_rule', _('Правила клиентов'));
		s.anonymous = false;
		s.addremove = true;
		s.addbtntitle = _('Добавить правило');
		s.description = _('Include/exclude: nft mark. Full route: секция маршрутизации. Global exclude: direct без tproxy.');

		s.option(form.Value, 'ip', _('IP / CIDR клиента')).placeholder = '192.168.1.50';

		var mode = s.option(form.ListValue, 'mode', _('Режим'));
		for (var i = 0; i < MODES.length; i++)
			mode.value(MODES[i][0], MODES[i][1]);

		var sec = s.option(form.Value, 'section', _('Секция маршрутизации'));
		sec.placeholder = 'glob';
		sec.depends('mode', 'full_route');

		var leaseHint = s.option(form.DummyValue, '_dhcp_hint', _('DHCP leases'));
		leaseHint.render = function() {
			return E('button', {
				'class': 'btn cbi-button cbi-button-action',
				'click': function() {
					return callDhcpLeases().then(function(res) {
						var d = res.data || res;
						var leases = (d && d.dhcp && d.dhcp.leases) ? d.dhcp.leases : (d && d.leases) ? d.leases : [];
						if (!leases.length && d && d['dhcp.leases'])
							leases = d['dhcp.leases'];
						var lines = [];
						for (var i = 0; i < leases.length; i++) {
							var L = leases[i];
							lines.push((L.hostname || L.mac || '?') + ' → ' + (L.ipaddr || L.ip || ''));
						}
						ui.addNotification(null, E('pre', { 'style': 'white-space:pre-wrap;font-size:12px;' },
							lines.length ? lines.join('\n') : _('Нет leases или ubus dhcp недоступен')), 'info');
					});
				}
			}, _('Показать DHCP leases'));
		};

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
