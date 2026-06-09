'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';
'require hybrid-failover.hf-ui as hfui';

var callReload = rpc.declare({
	object: 'hybrid-failover',
	method: 'reload'
});

var MODES = [
	['include', _('Include: через Hybrid Failover (nft mark)')],
	['exclude', _('Exclude: миновать Hybrid Failover')],
	['full_route', _('Full route: весь трафик клиента через секцию')],
	['global_exclude', _('Global exclude: исключить из tproxy маршрутизации')]
];

var MODE_HINTS = {
	include: _('Трафик клиента проходит через sing-box tproxy и nft mark.'),
	exclude: _('Клиент не попадает под Hybrid Failover (direct).'),
	full_route: _('Весь трафик клиента направляется через выбранную секцию маршрутизации.'),
	global_exclude: _('Исключение из tproxy без per-section правила.')
};

function normalizeLeases(res) {
	var d = hfui.unwrapData(res) || res;
	var leases = (d && d.dhcp && d.dhcp.leases) ? d.dhcp.leases : (d && d.leases) ? d.leases : [];
	if (!leases.length && d && d['dhcp.leases'])
		leases = d['dhcp.leases'];
	return leases;
}

return view.extend({
	_effectiveEl: null,
	_effectiveHint: null,

	loadEffectiveClients: function() {
		var self = this;
		return hfui.rpc.listClients().then(function(res) {
			var d = hfui.unwrapData(res) || res;
			self._renderEffectiveTable(d);
			if (self._effectiveHint)
				self._effectiveHint.textContent = _('Обновлено') + ': ' + new Date().toLocaleTimeString();
		}).catch(function(err) {
			if (self._effectiveEl)
				self._effectiveEl.textContent = String(err.message || err);
		});
	},

	_renderEffectiveTable: function(data) {
		if (!this._effectiveEl)
			return;
		hfui.emptyNode(this._effectiveEl);
		var clients = (data && data.rules) ? data.rules : (data && data.clients) ? data.clients : (Array.isArray(data) ? data : []);
		if (!clients.length) {
			this._effectiveEl.appendChild(E('p', { 'class': 'hf-mon-empty' }, _('Нет effective rules (client_rule пуст или core не запущен).')));
			return;
		}
		var rows = clients.map(function(c) {
			var source = c.source || c.name || '-';
			if (/^legacy-/.test(source))
				source = 'legacy';
			else if (source && source !== '-')
				source = 'client_rule';
			return E('tr', {}, [
				E('td', {}, c.ip || c.ipaddr || '-'),
				E('td', {}, c.mode || '-'),
				E('td', {}, c.section || '-'),
				E('td', {}, source)
			]);
		});
		this._effectiveEl.appendChild(hfui.wrapTable([
			E('thead', {}, E('tr', {}, [
				E('th', {}, _('IP')),
				E('th', {}, _('Режим')),
				E('th', {}, _('Секция')),
				E('th', {}, _('Источник'))
			])),
			E('tbody', {}, rows)
		]));
	},

	showDhcpPicker: function() {
		var self = this;
		return hfui.rpc.dhcpLeases().then(function(res) {
			var leases = normalizeLeases(res);
			if (!leases.length) {
				ui.addNotification(null, E('p', {}, _('Нет leases или ubus dhcp недоступен')), 'info');
				return;
			}
			var tbody = E('tbody', {}, leases.map(function(L) {
				var ip = L.ipaddr || L.ip || '';
				var host = L.hostname || L.mac || '?';
				return E('tr', {}, [
					E('td', {}, host),
					E('td', {}, ip),
					E('td', {}, L.mac || '-'),
					E('td', {}, L.expires != null ? String(L.expires) : (L.leasetime || '-')),
					E('td', {}, E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': function() {
							document.body.querySelectorAll('.hf-mon-modal-backdrop').forEach(function(el) {
								el.parentNode.removeChild(el);
							});
							var ipInput = document.querySelector('input[name*="ip"]');
							if (ipInput)
								ipInput.value = ip;
							ui.addNotification(null, E('p', {}, _('IP подставлен в форму: ') + ip), 'info');
						}
					}, _('Добавить')))
				]);
			}));
			hfui.showModal(_('DHCP leases'), [
				hfui.wrapTable([
					E('thead', {}, E('tr', {}, [
						E('th', {}, _('Hostname')),
						E('th', {}, _('IP')),
						E('th', {}, _('MAC')),
						E('th', {}, _('Lease')),
						E('th', {}, '')
					])),
					tbody
				])
			]);
		});
	},

	renderEffectivePanel: function() {
		var self = this;
		this._effectiveEl = E('div', {});
		this._effectiveHint = E('span', { 'class': 'hint', 'style': 'font-size:12px;' }, '');
		var panel = E('div', { 'class': 'cbi-section hf-mon hf-mon-section' }, [
			E('h3', {}, _('Effective rules (как видит core)')),
			E('p', { 'class': 'hint' }, _('Read-only сводка list_clients. Изменения клиентов применяются сразу через reload, без pending.')),
			E('div', { 'class': 'hf-mon-toolbar' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() { return self.loadEffectiveClients(); })
				}, _('Обновить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(self, function() { return self.showDhcpPicker(); })
				}, _('Выбрать из DHCP…')),
				this._effectiveHint
			]),
			this._effectiveEl
		]);
		hfui.injectStyles(panel);
		return panel;
	},

	load: function() {
		return uci.load('hybrid-failover');
	},

	render: function() {
		var self = this;
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

		var modeHint = s.option(form.DummyValue, '_mode_hint', _('Описание режима'));
		modeHint.renderWidget = function(section_id, option_id, cfgvalue) {
			var modeVal = uci.get('hybrid-failover', section_id, 'mode') || 'include';
			return E('p', { 'class': 'hint', 'style': 'margin:0;' }, MODE_HINTS[modeVal] || '');
		};

		var sec = s.option(form.Value, 'section', _('Секция маршрутизации'));
		sec.placeholder = 'glob';
		sec.depends('mode', 'full_route');

		return m.render().then(function(node) {
			var panel = self.renderEffectivePanel();
			if (node && node.insertBefore)
				node.insertBefore(panel, node.firstChild);
			self.loadEffectiveClients();
			return node;
		});
	},

	_applyReload: function() {
		return callReload().then(function(res) {
			hfui.notifyRpcResult(_('Reload sing-box'), res);
			return res;
		});
	},

	handleSaveApply: function() {
		var self = this;
		return this.map.save(true).then(function() {
			return self._applyReload();
		});
	},

	handleSave: function() {
		var self = this;
		return this.map.save(false).then(function() {
			ui.addNotification(null, E('p', {}, _('Сохранено. Применяется reload sing-box…')), 'info');
			return self._applyReload();
		});
	}
});
