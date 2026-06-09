'use strict';
'require view';
'require poll';
'require ui';
'require hybrid-failover.hf-ui as hfui';

function exportHistoryFile() {
	return hfui.rpc.exportHistory().then(function(res) {
		var data = hfui.unwrapData(res);
		if (data == null)
			data = res;
		var blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
		var a = document.createElement('a');
		a.href = URL.createObjectURL(blob);
		a.download = 'failover-history.json';
		a.click();
		URL.revokeObjectURL(a.href);
	});
}

function primaryErrorForSection(data, section) {
	if (!data || !data.controller)
		return '';
	for (var i = 0; i < data.controller.length; i++) {
		var c = data.controller[i];
		if (c.section === section && c.last_error)
			return c.last_error;
	}
	return '';
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	healthBusy: false,
	_lastStatus: null,
	_lastHistory: null,
	_lastDelayHistory: null,
	_loadError: null,
	_channelData: null,
	_channelsProbed: false,
	_healthStarted: false,
	_pollCountdown: 5,
	_sectionFilter: '',
	_historyLimit: 15,
	_monRoot: null,
	_monUpdated: null,
	_switchSectionEl: null,
	_switchOutboundEl: null,

	load: function() {
		return Promise.all([
			hfui.rpc.status().then(function(res) {
				return { ok: true, data: hfui.unwrapData(res) };
			}).catch(function(err) {
				return { ok: false, error: err };
			}),
			hfui.rpc.history().then(function(res) {
				return { ok: true, data: hfui.unwrapData(res) };
			}).catch(function(err) {
				return { ok: false, error: err };
			}),
			hfui.rpc.delayHistory().then(function(res) {
				return { ok: true, data: hfui.unwrapData(res) };
			}).catch(function(err) {
				return { ok: false, error: err };
			})
		]);
	},

	renderMonitor: function() {
		var self = this;
		var root = this._monRoot;
		var updated = this._monUpdated;
		if (!root)
			return;

		var section = self._sectionFilter;
		if (!section && self._lastStatus) {
			if (self._lastStatus.failover && self._lastStatus.failover.section)
				section = self._lastStatus.failover.section;
			else if (self._lastStatus.controller && self._lastStatus.controller.length)
				section = self._lastStatus.controller[0].section;
		}

		var content;
		if (this._loadError) {
			content = hfui.buildErrorBanner(this._loadError);
		} else {
			var channels = this._channelData || (this._lastStatus && this._lastStatus.channels) || [];
			content = E('div', {}, [
				hfui.buildMetaLine(this._lastStatus),
				hfui.buildSummaryBanner(this._lastStatus, {
					primaryError: primaryErrorForSection(this._lastStatus, section)
				}),
				hfui.buildMetricCards(this._lastStatus),
				hfui.buildFailoverPanels(this._lastStatus, section),
				E('div', { 'class': 'hf-mon-section' }, [
					E('h3', {}, _('Каналы и задержки')),
					hfui.buildChannelsTable(channels, this._channelsProbed, this._lastDelayHistory)
				]),
				E('div', { 'class': 'hf-mon-section' }, [
					E('div', { 'style': 'display:flex;align-items:center;gap:12px;margin-bottom:8px;flex-wrap:wrap;' }, [
						E('h3', { 'style': 'margin:0;' }, _('История переключений')),
						E('label', { 'style': 'font-size:12px;' }, [
							_('Показать') + ' ',
							E('select', {
								'class': 'cbi-input-select',
								'change': ui.createHandlerFn(self, function(ev) {
									self._historyLimit = parseInt(ev.target.value, 10) || 15;
									self.renderMonitor();
								})
							}, [
								E('option', { 'value': '15', 'selected': self._historyLimit === 15 }, '15'),
								E('option', { 'value': '50', 'selected': self._historyLimit === 50 }, '50')
							])
						])
					]),
					hfui.buildHistoryTable(this._lastHistory || [], section, self._historyLimit)
				])
			]);
		}

		hfui.emptyNode(root);
		root.appendChild(content);
		if (updated)
			updated.textContent = _('Обновлено') + ': ' + new Date().toLocaleTimeString() +
				' · ' + _('следующее через') + ' ' + self._pollCountdown + 's';
	},

	updateSectionPicker: function() {
		var el = document.getElementById('hf-section-picker');
		if (!el || !this._lastStatus)
			return;
		var opts = hfui.sectionOptions(this._lastStatus.controller,
			this._lastStatus.failover && this._lastStatus.failover.section);
		while (el.firstChild)
			el.removeChild(el.firstChild);
		opts.forEach(function(s) {
			el.appendChild(E('option', { 'value': s, 'selected': s === this._sectionFilter }, s));
		}.bind(this));
		if (!this._sectionFilter && opts.length)
			this._sectionFilter = opts[0];
	},

	updateSwitchPanel: function() {
		var secEl = this._switchSectionEl;
		var outEl = this._switchOutboundEl;
		if (!secEl || !outEl)
			return;
		var data = this._lastStatus;
		var opts = hfui.sectionOptions(data && data.controller, data && data.failover && data.failover.section);
		while (secEl.firstChild)
			secEl.removeChild(secEl.firstChild);
		opts.forEach(function(s) {
			secEl.appendChild(E('option', { 'value': s, 'selected': s === this._sectionFilter }, s));
		}.bind(this));
		if (!this._sectionFilter && opts.length)
			this._sectionFilter = opts[0];
		while (outEl.firstChild)
			outEl.removeChild(outEl.firstChild);
		var channels = this._channelData || (data && data.channels) || [];
		var policy = data && data.failover && data.failover.policy;
		var fastest = policy === 'fastest';
		for (var i = 0; i < channels.length; i++) {
			var ch = channels[i];
			if (!ch.name)
				continue;
			outEl.appendChild(E('option', { 'value': ch.name }, ch.display || ch.name));
		}
		outEl.disabled = fastest;
	},

	applyLoadResults: function(results) {
		if (results[0] && results[0].ok) {
			this._lastStatus = results[0].data;
			this._loadError = null;
			hfui.recordDelayHistoryLocal(this._lastStatus.channels || []);
		} else {
			this._lastStatus = null;
			this._loadError = results[0] && results[0].error
				? String(results[0].error.message || results[0].error)
				: _('не удалось получить статус');
		}
		if (results[1] && results[1].ok)
			this._lastHistory = Array.isArray(results[1].data) ? results[1].data : [];
		else if (!this._lastHistory)
			this._lastHistory = [];
		if (results[2] && results[2].ok)
			this._lastDelayHistory = results[2].data;
	},

	refreshAll: function() {
		var self = this;
		return self.load().then(function(results) {
			self.applyLoadResults(results);
			self.renderMonitor();
			self.updateSectionPicker();
			self.updateSwitchPanel();
			if (!self._loadError && !self._healthStarted && self._lastStatus && self._lastStatus.clash_ok)
				return self.runHealthProbe();
		}).catch(function(err) {
			self._loadError = String(err.message || err);
			self.renderMonitor();
		});
	},

	runHealthProbe: function() {
		var self = this;
		if (self.healthBusy)
			return Promise.resolve();
		self.healthBusy = true;
		var btn = document.getElementById('hf-btn-probe');
		if (btn)
			btn.disabled = true;
		return hfui.rpc.health().then(function(res) {
			var data = hfui.unwrapData(res);
			self._channelData = data && data.channels;
			self._channelsProbed = true;
			self._healthStarted = true;
			if (data && self._lastStatus) {
				if (data.channels) {
					self._lastStatus.channels = data.channels;
					hfui.recordDelayHistoryLocal(data.channels);
				}
				if (data.controller)
					self._lastStatus.controller = data.controller;
				if (data.failover)
					self._lastStatus.failover = data.failover;
			}
			return hfui.rpc.delayHistory().then(function(dh) {
				self._lastDelayHistory = hfui.unwrapData(dh);
			}).catch(function() {});
		}).then(function() {
			self.renderMonitor();
			self.updateSwitchPanel();
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		}).finally(function() {
			self.healthBusy = false;
			if (btn)
				btn.disabled = false;
		});
	},

	render: function(loadResults) {
		var self = this;
		self.applyLoadResults(loadResults || [{ ok: false }, { ok: false }, { ok: false }]);

		var root = E('div', { 'id': 'hf-mon-root' });
		self._monRoot = root;
		self.renderMonitor();

		var box = E('div', { 'class': 'cbi-section hf-mon' }, [
			E('h2', {}, _('Hybrid Failover: обзор')),
			E('p', { 'class': 'hint' },
				_('Сводка состояния, каналы failover, контроллер политики и журнал переключений. Обновление каждые 5 с.')),
			E('div', { 'class': 'hf-mon-toolbar' }, [
				E('label', { 'style': 'font-size:12px;' }, [
					_('Секция') + ' ',
					E('select', {
						'id': 'hf-section-picker',
						'class': 'cbi-input-select',
						'change': ui.createHandlerFn(self, function(ev) {
							self._sectionFilter = ev.target.value;
							self.renderMonitor();
							self.updateSwitchPanel();
						})
					})
				]),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'id': 'hf-btn-refresh',
					'click': ui.createHandlerFn(self, function() { return self.refreshAll(); })
				}, _('Обновить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'id': 'hf-btn-probe',
					'click': ui.createHandlerFn(self, function() { return self.runHealthProbe(); })
				}, _('Live probe')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return hfui.rpc.globalCheck().then(function(res) {
							var items = hfui.parseChecklist(res);
							ui.addNotification(null, hfui.renderChecklist(items), items.every(function(x) { return x.ok; }) ? 'info' : 'danger');
						});
					})
				}, _('global-check')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return hfui.rpc.checkFakeip().then(function(res) {
							var ok = res && res.ok !== false && !(res.data && res.data.ok === false);
							ui.addNotification(null, E('p', {},
								(res && res.data && res.data.message) || (res && res.output) || _('Готово')),
								ok ? 'info' : 'danger');
						});
					})
				}, _('check-fakeip')),
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(self, function() {
						return exportHistoryFile().catch(function(err) {
							ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
						});
					})
				}, _('Экспорт журнала')),
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(self, function() {
						hfui.clearDelayHistoryLocal();
						ui.addNotification(null, E('p', {}, _('Графики задержек сброшены')), 'info');
						return Promise.resolve();
					})
				}, _('Сбросить графики')),
				E('span', { 'class': 'hf-mon-updated', 'id': 'hf-mon-updated' }, '')
			]),
			E('div', { 'class': 'hf-mon-section', 'id': 'hf-switch-block' }, [
				E('h3', {}, _('Ручное переключение канала')),
				E('div', { 'class': 'hf-mon-switch' }, [
					E('div', {}, [
						E('label', {}, _('Секция')),
						E('select', { 'id': 'hf-switch-section', 'class': 'cbi-input-select',
							'change': ui.createHandlerFn(self, function(ev) {
								self._sectionFilter = ev.target.value;
								self.renderMonitor();
							})
						})
					]),
					E('div', {}, [
						E('label', {}, _('Outbound')),
						E('select', { 'id': 'hf-switch-outbound', 'class': 'cbi-input-select' })
					]),
					E('button', {
						'class': 'btn cbi-button cbi-button-apply',
						'click': ui.createHandlerFn(self, function() {
							var section = document.getElementById('hf-switch-section');
							var outbound = document.getElementById('hf-switch-outbound');
							if (!section || !outbound || !outbound.value)
								return Promise.resolve();
							var fo = self._lastStatus && self._lastStatus.failover;
							if (fo && fo.policy === 'fastest') {
								ui.addNotification(null, E('p', {}, _('При policy fastest ручное переключение недоступно')), 'warning');
								return Promise.resolve();
							}
							hfui.showModal(_('Подтверждение'), [
								E('p', {}, _('Переключить selector ') + section.value + ':'),
								E('p', { 'class': 'hf-mon-tag' }, outbound.value)
							], function() {
								hfui.rpc.switchProxy(section.value, outbound.value).then(function(res) {
									var ok = res && res.ok !== false && !(res.data && res.data.ok === false);
									ui.addNotification(null, E('p', {},
										ok ? _('Переключено') : (res.data && res.data.error) || _('Ошибка')),
										ok ? 'info' : 'danger');
									if (ok)
										return self.refreshAll();
								}).catch(function(err) {
									ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
								});
							});
							return Promise.resolve();
						})
					}, _('Переключить'))
				])
			]),
			root
		]);

		hfui.injectStyles(box);
		self._monUpdated = box.querySelector('#hf-mon-updated');
		self._switchSectionEl = document.getElementById('hf-switch-section');
		self._switchOutboundEl = document.getElementById('hf-switch-outbound');
		self.updateSectionPicker();
		self.updateSwitchPanel();

		poll.add(function() {
			self._pollCountdown = 5;
			return self.refreshAll().then(function() {
				self._pollCountdown = 5;
			});
		}, 5);
		poll.add(function() {
			if (self._pollCountdown > 0)
				self._pollCountdown--;
			self.renderMonitor();
		}, 1);

		return box;
	}
});
