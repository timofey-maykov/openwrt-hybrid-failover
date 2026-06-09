'use strict';
'require view';
'require form';
'require fs';
'require rpc';
'require uci';
'require ui';
'require hybrid-failover.hf-ui as hfui';

var fieldStyle = 'width:100%;max-width:100%;box-sizing:border-box;';

var callServiceList = rpc.declare({
	object: 'service',
	method: 'list',
	params: [ 'name' ]
});

var callServiceRestart = rpc.declare({
	object: 'service',
	method: 'restart',
	params: [ 'name' ]
});

var BOT_SERVICE = 'hybrid-failover-bot';

return view.extend({
	configFile: '/etc/hybrid-failover-bot.json',
	botBinary: '/usr/bin/hybrid-failover-bot',
	_actionResultEl: null,
	_serviceBadgeEl: null,

	handleAction: function(mode, title) {
		var self = this;
		return fs.exec(this.botBinary, [
			'-mode', mode,
			'-config', this.configFile
		]).then(function(res) {
			var output = (res.stdout || '').trim() || (res.stderr || '').trim() || _('Готово');
			if (self._actionResultEl)
				self._actionResultEl.textContent = (title || mode) + '\n' + output;
			ui.addNotification(null, E('p', {}, output), res.code === 0 ? 'info' : 'danger');
		}).catch(function(err) {
			var msg = String(err.message || err);
			if (self._actionResultEl)
				self._actionResultEl.textContent = (title || mode) + '\n' + msg;
			ui.addNotification(null, E('p', {}, _('Ошибка: ') + msg), 'danger');
		});
	},

	refreshServiceStatus: function() {
		var self = this;
		return callServiceList(BOT_SERVICE).then(function(res) {
			var svc = res && res[BOT_SERVICE];
			var running = false;
			var pid = '';
			if (svc && svc.instances) {
				for (var k in svc.instances) {
					if (svc.instances[k].running) {
						running = true;
						if (svc.instances[k].pid)
							pid = String(svc.instances[k].pid);
						break;
					}
				}
			}
			if (self._serviceBadgeEl) {
				var label = running ? _('running') : _('stopped');
				if (running && pid)
					label += ' (pid ' + pid + ')';
				self._serviceBadgeEl.textContent = label;
				self._serviceBadgeEl.className = 'hf-mon-badge ' + (running ? 'hf-mon-badge--ok' : 'hf-mon-badge--bad');
			}
		}).catch(function() {
			if (self._serviceBadgeEl) {
				self._serviceBadgeEl.textContent = _('недоступен');
				self._serviceBadgeEl.className = 'hf-mon-badge hf-mon-badge--warn';
			}
		});
	},

	setPending: function(key, value) {
		return fs.exec(this.botBinary, [
			'-mode', 'set-pending',
			'-config', this.configFile,
			'-key', key,
			'-value', String(value != null ? value : '')
		]);
	},

	savePendingFromForm: function() {
		var get = function(id) {
			var el = document.getElementById(id);
			return el ? el.value : '';
		};
		var tasks = [
			['token', get('pdkb_token')],
			['admin_ids', get('pdkb_admin_ids')],
			['viewer_ids', get('pdkb_viewer_ids')],
			['policy', get('pdkb_policy')],
			['clash_api', get('pdkb_clash_api')],
			['routing_init_script', get('pdkb_routing_init_script')],
			['log_path', get('pdkb_log_path')],
			['audit_path', get('pdkb_audit_path')],
			['probe_timeout_seconds', get('pdkb_probe_timeout_seconds')],
			['notify_failover_enabled', get('pdkb_notify_failover_enabled')],
			['notify_failover_interval_seconds', get('pdkb_notify_interval')]
		];
		var self = this;
		var chain = Promise.resolve();
		tasks.forEach(function(kv) {
			chain = chain.then(function() {
				return self.setPending(kv[0], kv[1]);
			});
		});
		return chain.then(function() {
			ui.addNotification(null, E('p', {}, _('Сохранено в pending-конфиг. Нажмите «Проверить» или «Применить».')));
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, _('Ошибка сохранения: ') + (err.message || err)), 'danger');
		});
	},

	renderEditor: function(cfg) {
		var self = this;
		var mkInput = function(label, id, value, attrs) {
			var a = attrs || {};
			return E('div', { 'class': 'cbi-value', 'style': 'margin-bottom:12px;width:100%;' }, [
				E('label', { 'class': 'cbi-value-title', 'for': id, 'style': 'display:block;font-weight:600;margin-bottom:6px;' }, label),
				E('div', { 'class': 'cbi-value-field' }, [
					E('input', Object.assign({
						'id': id,
						'class': 'cbi-input-text',
						'type': 'text',
						'value': value || '',
						'style': fieldStyle
					}, a))
				])
			]);
		};

		var policy = cfg.policy || 'outage-only';
		return E('div', { 'class': 'cbi-section hf-mon-panel', 'style': 'width:100%;margin-top:16px;' }, [
			E('h3', {}, _('Hybrid Failover Bot: JSON-конфиг')),
			E('p', { 'class': 'hint' }, hfui.policyHint(policy)),
			mkInput(_('Токен'), 'pdkb_token', cfg.token || '', { 'placeholder': '123456789:ABC...' }),
			mkInput(_('ID администраторов (через запятую)'), 'pdkb_admin_ids', (cfg.admin_ids || []).join(', '), { 'placeholder': '123456789, 987654321' }),
			mkInput(_('ID только чтение (viewer_ids)'), 'pdkb_viewer_ids', (cfg.viewer_ids || []).join(', '), { 'placeholder': '111111111' }),
			E('div', { 'class': 'cbi-value', 'style': 'margin-bottom:12px;width:100%;' }, [
				E('label', { 'class': 'cbi-value-title', 'for': 'pdkb_policy', 'style': 'display:block;font-weight:600;margin-bottom:6px;' }, _('Политика failover')),
				E('div', { 'class': 'cbi-value-field' }, [
					E('select', { 'id': 'pdkb_policy', 'class': 'cbi-input-select', 'style': fieldStyle }, [
						E('option', { 'value': 'outage-only', 'selected': policy === 'outage-only' }, _('outage-only (только при падении)')),
						E('option', { 'value': 'prefer-primary', 'selected': policy === 'prefer-primary' }, _('prefer-primary (предпочитать основной)')),
						E('option', { 'value': 'fastest', 'selected': policy === 'fastest' }, _('fastest (urltest passive)'))
					])
				])
			]),
			mkInput(_('URL Clash API'), 'pdkb_clash_api', cfg.clash_api || 'http://192.168.42.1:9090'),
			mkInput(_('Скрипт init.d hybrid-failover'), 'pdkb_routing_init_script',
				cfg.routing_init_script || '/etc/init.d/hybrid-failover'),
			mkInput(_('Путь к логам'), 'pdkb_log_path', cfg.log_path || '/var/log/hybrid-failover-bot.log'),
			mkInput(_('Путь к audit-логу'), 'pdkb_audit_path', cfg.audit_path || '/var/log/hybrid-failover-bot.audit.log'),
			mkInput(_('Таймаут проверки, сек'), 'pdkb_probe_timeout_seconds', String(cfg.probe_timeout_seconds || 5), { 'type': 'number', 'min': '1', 'style': 'max-width:200px;width:100%;' }),
			mkInput(_('Алерты failover (true/false)'), 'pdkb_notify_failover_enabled', cfg.notify_failover_enabled ? 'true' : 'false'),
			mkInput(_('Интервал алертов, сек'), 'pdkb_notify_interval', String(cfg.notify_failover_interval_seconds || 30), { 'type': 'number', 'min': '10' }),
			E('div', { 'style': 'display:flex;gap:8px;flex-wrap:wrap;margin-top:12px;' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'click': ui.createHandlerFn(this, function() { return this.savePendingFromForm(); })
				}, _('Сохранить в pending'))
			])
		]);
	},

	renderActionsPanel: function() {
		var self = this;
		this._actionResultEl = E('pre', {
			'class': 'hf-step-result',
			'style': 'margin-top:12px;'
		}, '-');
		return E('div', { 'class': 'cbi-section hf-mon-panel', 'style': 'width:100%;' }, [
			E('h3', {}, _('Действия с конфигом')),
			E('div', { 'class': 'hf-mon-stepper' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, function() { return this.handleAction('validate-config', _('Проверить pending')); })
				}, _('Проверить pending')),
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'click': ui.createHandlerFn(this, function() { return this.handleAction('apply-config', _('Применить')); })
				}, _('Применить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-negative',
					'click': ui.createHandlerFn(this, function() { return this.handleAction('rollback-config', _('Откатить')); })
				}, _('Откатить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, function() {
						return callServiceRestart(BOT_SERVICE).then(function() {
							return self.refreshServiceStatus();
						}).then(function() {
							if (self._actionResultEl)
								self._actionResultEl.textContent = _('Перезапуск') + '\n' + _('Готово');
						}).catch(function(err) {
							var msg = String(err.message || err);
							if (self._actionResultEl)
								self._actionResultEl.textContent = _('Перезапуск') + '\n' + msg;
							ui.addNotification(null, E('p', {}, msg), 'danger');
						});
					})
				}, _('Перезапустить бота'))
			]),
			this._actionResultEl
		]);
	},

	render: function() {
		var m, s, o;
		var self = this;

		m = new form.Map('hybrid-failover-bot', _('Telegram-бот Hybrid Failover'),
			_('Настройка бота, pending-конфиг и безопасное применение изменений.'));

		s = m.section(form.NamedSection, 'main', 'bot', _('Сервис'));
		s.anonymous = true;

		o = s.option(form.Flag, 'enabled', _('Включить сервис'));
		o.default = o.disabled;

		o = s.option(form.Value, 'binary', _('Путь к бинарнику'));
		o.datatype = 'string';
		o.default = '/usr/bin/hybrid-failover-bot';
		o.width = '100%';

		o = s.option(form.Value, 'config_path', _('Путь к конфигу JSON'));
		o.datatype = 'string';
		o.default = '/etc/hybrid-failover-bot.json';
		o.width = '100%';

		o = s.option(form.Value, 'log_path', _('Путь к лог-файлу'));
		o.datatype = 'string';
		o.default = '/var/log/hybrid-failover-bot.log';
		o.width = '100%';

		self._serviceBadgeEl = E('span', { 'class': 'hf-mon-badge hf-mon-badge--info' }, '…');
		s = m.section(form.TypedSection, 'bot_status', _('Статус сервиса'));
		s.anonymous = true;
		s.render = function() {
			return E('div', { 'class': 'hf-mon-toolbar' }, [
				E('span', {}, _('init.d hybrid-failover-bot:')),
				self._serviceBadgeEl,
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() { return self.refreshServiceStatus(); })
				}, _('Обновить'))
			]);
		};

		return fs.read(this.configFile).then(function(raw) {
			var cfg = {};
			try { cfg = JSON.parse(raw || '{}'); } catch (e) { cfg = {}; }
			return m.render().then(function(mapNode) {
				var wrap = E('div', { 'class': 'hf-mon pdkb-luci-wide', 'style': 'width:100%;max-width:100%;' }, [
					mapNode,
					self.renderActionsPanel(),
					self.renderEditor(cfg)
				]);
				hfui.injectStyles(wrap);
				self.refreshServiceStatus();
				return wrap;
			});
		}).catch(function() {
			return m.render().then(function(mapNode) {
				var wrap = E('div', { 'class': 'hf-mon' }, [ mapNode, self.renderActionsPanel() ]);
				hfui.injectStyles(wrap);
				self.refreshServiceStatus();
				return wrap;
			});
		});
	}
});
