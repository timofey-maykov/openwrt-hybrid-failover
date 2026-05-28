'use strict';
'require view';
'require form';
'require fs';
'require uci';
'require ui';

var fieldStyle = 'width:100%;max-width:100%;box-sizing:border-box;';

return view.extend({
	configFile: '/etc/podkop-telegram-bot.json',
	botBinary: '/usr/bin/podkop-telegram-bot',

	handleAction: function(mode) {
		return fs.exec(this.botBinary, [
			'-mode', mode,
			'-config', this.configFile
		]).then(function(res) {
			var output = (res.stdout || '').trim();
			ui.addNotification(null, E('p', output || 'Готово'));
		}).catch(function(err) {
			ui.addNotification(null, E('p', 'Ошибка: ' + (err.message || err)));
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
			['policy', get('pdkb_policy')],
			['clash_api', get('pdkb_clash_api')],
			['podkop_init_script', get('pdkb_podkop_init_script')],
			['log_path', get('pdkb_log_path')],
			['audit_path', get('pdkb_audit_path')],
			['probe_timeout_seconds', get('pdkb_probe_timeout_seconds')]
		];
		var self = this;
		var chain = Promise.resolve();
		tasks.forEach(function(kv) {
			chain = chain.then(function() {
				return self.setPending(kv[0], kv[1]);
			});
		});
		return chain.then(function() {
			ui.addNotification(null, E('p', 'Сохранено в pending-конфиг. Нажмите «Проверить» или «Применить».'));
		}).catch(function(err) {
			ui.addNotification(null, E('p', 'Ошибка сохранения: ' + (err.message || err)));
		});
	},

	renderEditor: function(cfg) {
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

		var mkSelect = function(label, id, options, selected) {
			return E('div', { 'class': 'cbi-value', 'style': 'margin-bottom:12px;width:100%;' }, [
				E('label', { 'class': 'cbi-value-title', 'for': id, 'style': 'display:block;font-weight:600;margin-bottom:6px;' }, label),
				E('div', { 'class': 'cbi-value-field' }, [
					E('select', {
						'id': id,
						'class': 'cbi-input-select',
						'style': fieldStyle
					}, options)
				])
			]);
		};

		return E('div', { 'class': 'cbi-section', 'style': 'width:100%;' }, [
			E('h3', 'Подкоп Telegram Bot: JSON-конфиг (редактирование)'),
			mkInput('Токен', 'pdkb_token', cfg.token || '', { 'placeholder': '123456789:ABC...' }),
			mkInput('ID администраторов (через запятую)', 'pdkb_admin_ids', (cfg.admin_ids || []).join(', '), { 'placeholder': '123456789, 987654321' }),
			mkSelect('Политика failover', 'pdkb_policy', [
				E('option', { 'value': 'outage-only', 'selected': cfg.policy === 'outage-only' }, 'outage-only (только при падении)'),
				E('option', { 'value': 'prefer-primary', 'selected': cfg.policy === 'prefer-primary' }, 'prefer-primary (предпочитать основной)')
			], cfg.policy),
			mkInput('URL Clash API', 'pdkb_clash_api', cfg.clash_api || 'http://192.168.42.1:9090'),
			mkInput('Скрипт init.d Podkop', 'pdkb_podkop_init_script', cfg.podkop_init_script || '/etc/init.d/podkop'),
			mkInput('Путь к логам', 'pdkb_log_path', cfg.log_path || '/var/log/podkop-telegram-bot.log'),
			mkInput('Путь к audit-логу', 'pdkb_audit_path', cfg.audit_path || '/var/log/podkop-telegram-bot.audit.log'),
			mkInput('Таймаут проверки, сек', 'pdkb_probe_timeout_seconds', String(cfg.probe_timeout_seconds || 5), { 'type': 'number', 'min': '1', 'style': 'max-width:200px;width:100%;' }),
			E('div', { 'style': 'display:flex;gap:8px;flex-wrap:wrap;margin-top:12px;' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'click': ui.createHandlerFn(this, function() { return this.savePendingFromForm(); })
				}, 'Сохранить в pending')
			])
		]);
	},

	render: function() {
		var m, s, o;
		var self = this;

		m = new form.Map('podkop-telegram-bot', 'Telegram-бот Podkop',
			'Настройка бота, pending-конфиг и безопасное применение изменений.');

		s = m.section(form.NamedSection, 'main', 'bot', 'Сервис');
		s.anonymous = true;

		o = s.option(form.Flag, 'enabled', 'Включить сервис');
		o.default = o.disabled;

		o = s.option(form.Value, 'binary', 'Путь к бинарнику');
		o.datatype = 'string';
		o.default = '/usr/bin/podkop-telegram-bot';
		o.width = '100%';

		o = s.option(form.Value, 'config_path', 'Путь к конфигу JSON');
		o.datatype = 'string';
		o.default = '/etc/podkop-telegram-bot.json';
		o.width = '100%';

		o = s.option(form.Value, 'log_path', 'Путь к лог-файлу');
		o.datatype = 'string';
		o.default = '/var/log/podkop-telegram-bot.log';
		o.width = '100%';

		s = m.section(form.TypedSection, 'bot_actions', 'Действия с конфигом');
		s.anonymous = true;
		s.render = function() {
			var container = E('div', { 'class': 'cbi-section', 'style': 'width:100%;' }, [
				E('h3', 'Действия с конфигом'),
				E('div', { 'style': 'display:flex;gap:8px;flex-wrap:wrap;' }, [
					E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, function() { return this.handleAction('validate-config'); }.bind(this))
					}, 'Проверить pending'),
					E('button', {
						'class': 'btn cbi-button cbi-button-save',
						'click': ui.createHandlerFn(this, function() { return this.handleAction('apply-config'); }.bind(this))
					}, 'Применить'),
					E('button', {
						'class': 'btn cbi-button cbi-button-negative',
						'click': ui.createHandlerFn(this, function() { return this.handleAction('rollback-config'); }.bind(this))
					}, 'Откатить'),
					E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': ui.createHandlerFn(this, function() { return fs.exec('/etc/init.d/podkop-telegram-bot', ['restart']); })
					}, 'Перезапустить бота')
				])
			]);
			return container;
		}.bind(this);

		var wideCss = E('style', { 'type': 'text/css' }, [
			'.pdkb-luci-wide .cbi-value-field input.cbi-input-text,',
			'.pdkb-luci-wide .cbi-value-field select.cbi-input-select {',
			'width:100% !important;max-width:100% !important;box-sizing:border-box;',
			'}',
			'.pdkb-luci-wide .cbi-value {width:100%;}',
			'.pdkb-luci-wide .cbi-map {width:100%;}'
		].join('\n'));

		return fs.read(this.configFile).then(function(raw) {
			var cfg = {};
			try { cfg = JSON.parse(raw || '{}'); } catch (e) { cfg = {}; }
			return m.render().then(function(mapNode) {
				return E('div', { 'class': 'pdkb-luci-wide', 'style': 'width:100%;max-width:100%;' }, [
					wideCss,
					mapNode,
					self.renderEditor(cfg)
				]);
			});
		}).catch(function() {
			return m.render();
		});
	}
});
