'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';

var callStatus = rpc.declare({
	object: 'curfew',
	method: 'status'
});

var callApply = rpc.declare({
	object: 'curfew',
	method: 'apply'
});

var callSync = rpc.declare({
	object: 'curfew',
	method: 'sync'
});

var callDhcpLeases = rpc.declare({
	object: 'curfew',
	method: 'dhcp_leases'
});

function unwrap(res) {
	if (!res)
		return null;
	if (res.data != null)
		return res.data;
	return res;
}

return view.extend({
	_statusEl: null,
	_statusHint: null,

	loadStatus: function() {
		var self = this;
		return callStatus().then(function(res) {
			self._renderStatus(unwrap(res));
		}).catch(function(err) {
			if (self._statusEl)
				self._statusEl.textContent = String(err.message || err);
		});
	},

	_renderStatus: function(st) {
		if (!this._statusEl)
			return;

		if (!st) {
			this._statusEl.textContent = _('Нет данных');
			return;
		}

		var lines = [];
		var active = st.active ? _('активен') : _('неактивен');
		var windowText = st.in_window ? _('в окне расписания') : _('вне окна расписания');
		lines.push(_('Лимит') + ': ' + active + ' (' + windowText + ')');
		lines.push(_('По умолчанию') + ': ' + (st.rate_limit || '?') + ', burst ' + (st.burst || '?'));
		lines.push(_('Расписание') + ': ' + (st.block_time || '?') + ' – ' + (st.unblock_time || '?') + ' ' + (st.timezone || ''));

		var targets = Array.isArray(st.targets) ? st.targets : [];
		if (targets.length) {
			lines.push('');
			lines.push(_('Устройства') + ':');
			targets.forEach(function(t) {
				var online = t.online ? _('online') : _('offline');
				var speed = (t.rate_limit || st.rate_limit || '?') + ', burst ' + (t.burst || st.burst || '?');
				lines.push('  ' + (t.name || '?') + ' ' + (t.ip || '') + ' ' + speed + ' ' + online + (t.mac ? ' (' + t.mac + ')' : ''));
			});
		} else {
			lines.push('');
			lines.push(_('Список устройств пуст.'));
		}

		this._statusEl.textContent = lines.join('\n');
		if (this._statusHint)
			this._statusHint.textContent = _('Обновлено') + ': ' + new Date().toLocaleTimeString();
	},

	showLeasePicker: function() {
		var self = this;
		return callDhcpLeases().then(function(res) {
			var data = unwrap(res) || {};
			var leases = Array.isArray(data.leases) ? data.leases : [];
			if (!leases.length) {
				ui.addNotification(null, E('p', {}, _('DHCP-аренды не найдены')), 'warning');
				return;
			}

			var rows = leases.map(function(l) {
				var label = (l.name || l.ip || l.mac || '?');
				if (l.ip)
					label += ' (' + l.ip + ')';
				return E('tr', { 'class': 'tr' }, [
					E('td', { 'class': 'td' }, label),
					E('td', { 'class': 'td right' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(self, function() {
								self._fillDeviceFromLease(l);
							})
						}, _('Выбрать'))
					])
				]);
			});

			ui.showModal(_('DHCP-клиенты'), [
				E('div', { 'class': 'table' }, [
					E('table', { 'class': 'table' }, [
						E('tbody', {}, rows)
					])
				]),
				E('div', { 'class': 'right' }, [
					E('button', {
						'class': 'btn',
						'click': ui.hideModal
					}, _('Закрыть'))
				])
			]);
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	_fillDeviceFromLease: function(lease) {
		var nameInput = document.querySelector('input[id*="device"][id*=".name"], input[name*=".name"]');
		var ipInput = document.querySelector('input[id*="device"][id*=".ip"], input[name*=".ip"]');
		if (!nameInput || !ipInput) {
			var addBtn = document.querySelector('.cbi-section-table .cbi-button-add, .cbi-button-add');
			if (addBtn)
				addBtn.click();
			nameInput = document.querySelector('input[name*=".name"]');
			ipInput = document.querySelector('input[name*=".ip"]');
		}
		if (!ipInput) {
			ui.addNotification(null, E('p', {}, _('Не найдена форма устройства')), 'danger');
			return;
		}
		if (lease.name && nameInput)
			nameInput.value = lease.name;
		if (lease.ip)
			ipInput.value = lease.ip;
		ipInput.dispatchEvent(new Event('change', { bubbles: true }));
		ui.hideModal();
	},

	render: function() {
		var self = this;
		var m, s, o, dev;

		m = new form.Map('curfew', _('Curfew'), _('Ночной лимит скорости для выбранных LAN-устройств (tc на br-lan).'));

		s = m.section(form.NamedSection, 'curfew', 'curfew', _('Настройки'));
		s.addremove = false;

		o = s.option(form.Flag, 'enabled', _('Включено'));
		o.default = '0';
		o.rmempty = false;

		o = s.option(form.Value, 'block_time', _('Начало лимита'));
		o.placeholder = '01:30';
		o.datatype = 'string';
		o.rmempty = false;

		o = s.option(form.Value, 'unblock_time', _('Конец лимита'));
		o.placeholder = '08:00';
		o.datatype = 'string';
		o.rmempty = false;

		o = s.option(form.Value, 'rate_limit', _('Скорость по умолчанию'));
		o.placeholder = '128kbit';
		o.description = _('Используется, если у устройства не задана своя скорость. Формат tc: 64kbit, 128kbit, 1mbit');
		o.rmempty = false;

		o = s.option(form.Value, 'burst', _('Burst по умолчанию'));
		o.placeholder = '16k';
		o.description = _('Burst по умолчанию для устройств без своего значения');
		o.rmempty = false;

		o = s.option(form.Value, 'lan_if', _('Интерфейс LAN'));
		o.placeholder = 'br-lan';
		o.rmempty = false;

		dev = m.section(form.TypedSection, 'device', _('Устройства'));
		dev.anonymous = true;
		dev.addremove = true;

		o = dev.option(form.Value, 'name', _('Имя DHCP'));
		o.placeholder = 'iPhone';
		o.description = _('Hostname из DHCP (поле name в аренде)');
		o.rmempty = false;

		o = dev.option(form.Value, 'ip', _('IP'));
		o.datatype = 'ip4addr';
		o.placeholder = '192.168.1.100';
		o.rmempty = false;

		o = dev.option(form.Value, 'rate_limit', _('Скорость'));
		o.placeholder = '128kbit';
		o.description = _('Пусто: использовать значение по умолчанию из настроек');
		o.rmempty = true;

		o = dev.option(form.Value, 'burst', _('Burst'));
		o.placeholder = '16k';
		o.description = _('Пусто: использовать burst по умолчанию');
		o.rmempty = true;

		return m.render().then(function(mapEl) {
			var view = self;
			var statusNode = E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, _('Статус')),
				E('div', { 'class': 'cbi-section-descr' }, _('Текущее состояние tc и привязка IP.')),
				E('div', { 'class': 'cbi-section-node' }, [
					E('pre', {
						'id': 'curfew-status-pre',
						'style': 'white-space:pre-wrap;margin:0;padding:0.5em;background:#f7f7f7;border:1px solid #ddd;'
					}),
					E('div', { 'class': 'right', 'style': 'margin-top:0.5em;' }, [
						E('button', {
							'class': 'cbi-button cbi-button-action',
							'click': ui.createHandlerFn(view, function() {
								return view.loadStatus();
							})
						}, _('Обновить')),
						' ',
						E('button', {
							'class': 'cbi-button cbi-button-apply',
							'click': ui.createHandlerFn(view, function() {
								return callSync().then(function() {
									ui.addNotification(null, E('p', {}, _('Синхронизация выполнена')), 'info');
									return view.loadStatus();
								}).catch(function(err) {
									ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
								});
							})
						}, _('Sync tc'))
					]),
					E('p', { 'id': 'curfew-status-hint', 'class': 'cbi-section-descr' })
				])
			]);
			view._statusEl = statusNode.querySelector('#curfew-status-pre');
			view._statusHint = statusNode.querySelector('#curfew-status-hint');

			var dhcpNode = E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, _('DHCP')),
				E('div', { 'class': 'cbi-section-descr' }, _('Подставить имя и IP из активных DHCP-аренд в новую строку устройства.')),
				E('div', { 'class': 'cbi-section-node' }, [
					E('button', {
						'class': 'cbi-button cbi-button-action',
						'click': ui.createHandlerFn(view, function() {
							return view.showLeasePicker();
						})
					}, _('Выбрать из DHCP'))
				])
			]);

			mapEl.appendChild(statusNode);
			mapEl.appendChild(dhcpNode);
			view.loadStatus();
			return mapEl;
		});
	},

	handleSaveApply: function(ev, mode) {
		var self = this;
		return this.super('handleSaveApply', [ev, mode]).then(function() {
			return callApply().then(function(res) {
				if (res && res.ok === false)
					throw new Error(res.output || _('apply failed'));
				ui.addNotification(null, E('p', {}, _('Настройки применены (cron, dnsmasq, tc)')), 'info');
				return self.loadStatus();
			});
		});
	}
});
