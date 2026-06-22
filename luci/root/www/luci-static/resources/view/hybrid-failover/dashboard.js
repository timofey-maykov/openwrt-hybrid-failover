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
	_lastRefreshTime: '',
	_sectionFilter: '',
	_historyLimit: 15,
	_activeTab: 'overview',
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

	buildSwitchCard: function() {
		var self = this;
		return E('div', { 'class': 'hf-ent-card', 'id': 'hf-switch-block' }, [
			E('p', { 'class': 'hf-ent-card__title' }, _('Ручное переключение')),
			E('div', { 'class': 'hf-mon-switch' }, [
				E('div', {}, [
					E('label', {}, _('Секция')),
					E('select', { 'id': 'hf-switch-section', 'class': 'cbi-input-select',
						'change': ui.createHandlerFn(self, function(ev) {
							self._sectionFilter = ev.target.value;
							var picker = document.getElementById('hf-section-picker');
							if (picker)
								picker.value = ev.target.value;
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
		]);
	},

	buildToolsPanel: function() {
		var self = this;
		return E('div', { 'class': 'hf-ent-card' }, [
			E('p', { 'class': 'hf-ent-card__title' }, _('Диагностика и данные')),
			E('div', { 'class': 'hf-ent-tools' }, [
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
							ui.addNotification(null, hfui.renderChecklist(items),
								items.every(function(x) { return x.ok; }) ? 'info' : 'danger');
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
				}, _('Сбросить графики'))
			])
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
			var reserveLabel = hfui.channelsReserveSummary(channels, this._lastStatus, this._channelsProbed);
			var tabs = [
				{ id: 'overview', label: _('Обзор') },
				{ id: 'channels', label: _('Каналы') + (reserveLabel ? ' (' + reserveLabel + ')' : '') },
				{ id: 'events', label: _('Журнал') },
				{ id: 'tools', label: _('Инструменты') }
			];
			var tabBar = hfui.buildTabBar(tabs, self._activeTab, function(id) {
				self._activeTab = id;
				self.renderMonitor();
			});
			var hero = hfui.buildStatusHero(this._lastStatus, {
				section: section,
				primaryError: primaryErrorForSection(this._lastStatus, section)
			});
			var tabBody;

			if (self._activeTab === 'channels') {
				tabBody = E('div', { 'class': 'hf-mon-section' }, [
					E('div', { 'class': 'hf-ent-section-head' }, [
						E('h3', {}, _('Каналы и задержки')),
						E('button', {
							'class': 'btn cbi-button cbi-button-save',
							'click': ui.createHandlerFn(self, function() { return self.runHealthProbe(); })
						}, _('Live probe'))
					]),
					hfui.buildChannelsTable(channels, this._channelsProbed, this._lastDelayHistory,
						hfui.isNativeEngine(this._lastStatus), this._lastStatus)
				]);
			} else if (self._activeTab === 'events') {
				tabBody = E('div', { 'class': 'hf-mon-section' }, [
					E('div', { 'class': 'hf-ent-section-head' }, [
						E('h3', {}, _('История переключений')),
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
				]);
			} else if (self._activeTab === 'tools') {
				tabBody = E('div', {}, [
					self.buildToolsPanel(),
					E('div', { 'style': 'margin-top:16px;' }, self.buildSwitchCard())
				]);
			} else {
				tabBody = E('div', { 'class': 'hf-ent-layout' }, [
					E('div', {}, [
						hfui.buildMetricCards(this._lastStatus, section),
						hfui.buildChannelsOverview(channels, this._lastStatus, this._channelsProbed, {
							section: section,
							onProbe: ui.createHandlerFn(self, function() {
								return self.runHealthProbe();
							})
						}),
						hfui.buildFailoverPanels(this._lastStatus, section)
					]),
					E('div', { 'class': 'hf-ent-sidebar' }, [
						self.buildSwitchCard()
					])
				]);
			}

			content = E('div', {}, [hero, tabBar, tabBody]);
		}

		hfui.emptyNode(root);
		root.appendChild(content);
		if (self._activeTab === 'overview' || self._activeTab === 'tools')
			self.updateSwitchPanel();
		var pillWrap = document.getElementById('hf-status-pill-wrap');
		if (pillWrap && self._lastStatus && !self._loadError) {
			hfui.emptyNode(pillWrap);
			pillWrap.appendChild(hfui.buildStatusPill(hfui.overallState(self._lastStatus)));
		}
		var metaEl = document.getElementById('hf-enterprise-meta');
		if (metaEl && !self._loadError) {
			hfui.emptyNode(metaEl);
			metaEl.appendChild(hfui.buildEnterpriseMeta(self._lastStatus));
		}
		if (updated) {
			self._lastRefreshTime = new Date().toLocaleTimeString();
			updated.textContent = _('Обновлено') + ': ' + self._lastRefreshTime +
				' · ' + _('следующее через') + ' ' + self._pollCountdown + 's';
		}
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
		var secEl = document.getElementById('hf-switch-section');
		var outEl = document.getElementById('hf-switch-outbound');
		if (!secEl || !outEl)
			return;
		this._switchSectionEl = secEl;
		this._switchOutboundEl = outEl;
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
			if (!self._loadError && !self._healthStarted && self._lastStatus &&
				hfui.controlOk(self._lastStatus))
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
		var btn = document.getElementById('hf-btn-probe') ||
			document.getElementById('hf-btn-probe-overview');
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
				if (data.active_outbound)
					self._lastStatus.active_outbound = data.active_outbound;
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

		var state = hfui.overallState(self._lastStatus);
		var box = E('div', { 'class': 'cbi-section hf-mon' }, [
			E('div', { 'class': 'hf-ent-top' }, [
				E('div', { 'class': 'hf-ent-top__brand' }, [
					E('h2', {}, _('Hybrid Failover')),
					E('div', { 'class': 'hf-ent-top__meta' }, [
						E('span', { 'id': 'hf-status-pill-wrap' }, hfui.buildStatusPill(state)),
						E('span', { 'id': 'hf-enterprise-meta' }, hfui.buildEnterpriseMeta(self._lastStatus))
					])
				]),
				E('div', { 'class': 'hf-ent-top__actions' }, [
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
					E('span', { 'class': 'hf-mon-updated', 'id': 'hf-mon-updated' }, '')
				])
			]),
			E('p', { 'class': 'hint', 'style': 'margin:-8px 0 16px;' },
				_('Сводка состояния, каналы failover, контроллер политики и журнал переключений. Обновление каждые 5 с.')),
			root
		]);

		hfui.injectStyles(box);
		self._monUpdated = box.querySelector('#hf-mon-updated');
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
			if (self._monUpdated)
				self._monUpdated.textContent = _('Обновлено') + ': ' +
					(self._lastRefreshTime || '-') +
					' · ' + _('следующее через') + ' ' + self._pollCountdown + 's';
		}, 1);

		return box;
	}
});
