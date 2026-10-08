'use strict';
'require view';
'require form';
'require fs';
'require rpc';
'require uci';
'require ui';
'require hybrid-failover.hf-ui as hfui';

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

// Keys the bot accepts through `-mode set-pending` (botconfig.SetPendingKey).
var FIELDS = [
	{ key: 'token', label: _('Токен бота'), hint: _('Выдаёт @BotFather при создании бота.'), secret: true, placeholder: '123456789:ABC…' },
	{ key: 'router_name', label: _('Имя роутера'), hint: _('Так роутер называется в уведомлениях и в панели бота. Пусто: имя хоста.'), placeholder: _('например, Дом') },
	{ key: 'admin_ids', label: _('Администраторы'), hint: _('Telegram ID через запятую. Им доступно всё управление. Свой ID можно узнать у @userinfobot.'), list: true, placeholder: '123456789, 987654321' },
	{ key: 'viewer_ids', label: _('Только просмотр'), hint: _('Эти пользователи видят состояние, но ничего не меняют.'), list: true, placeholder: '111111111' }
];

var ADVANCED = [
	{ key: 'clash_api', label: _('Адрес Clash API'), def: 'http://127.0.0.1:9090' },
	{ key: 'routing_init_script', label: _('Скрипт службы Hybrid Failover'), def: '/etc/init.d/hybrid-failover' },
	{ key: 'probe_timeout_seconds', label: _('Таймаут проверок, сек'), def: '5', number: true },
	{ key: 'log_path', label: _('Журнал бота'), def: '/var/log/hybrid-failover-bot.log' },
	{ key: 'audit_path', label: _('Журнал действий'), def: '/var/log/hybrid-failover-bot.audit.log' }
];

return view.extend({
	configFile: '/etc/hybrid-failover-bot.json',
	botBinary: '/usr/bin/hybrid-failover-bot',

	load: function() {
		return Promise.all([
			uci.load('hybrid-failover-bot'),
			L.resolveDefault(fs.read(this.configFile), '{}'),
			L.resolveDefault(callServiceList(BOT_SERVICE), null)
		]);
	},

	serviceState: function(res) {
		var svc = res && res[BOT_SERVICE];
		if (!svc)
			return null;
		for (var k in (svc.instances || {}))
			if (svc.instances[k].running)
				return true;
		return false;
	},

	statusPills: function(running) {
		var enabled = uci.get('hybrid-failover-bot', 'main', 'enabled') === '1';
		var pills = [];
		if (running === true)
			pills.push(hfui.pill(_('Бот работает'), 'ok'));
		else if (running === false)
			pills.push(hfui.pill(_('Бот остановлен'), 'bad'));
		else
			pills.push(hfui.pill(_('Состояние неизвестно'), ''));
		if (!enabled)
			pills.push(hfui.pill(_('Автозапуск выключен'), 'warn'));
		return pills;
	},

	refreshStatus: function() {
		var self = this;
		return L.resolveDefault(callServiceList(BOT_SERVICE), null).then(function(res) {
			self.header.setPills(self.statusPills(self.serviceState(res)));
		});
	},

	exec: function(args) {
		return fs.exec(this.botBinary, args.concat([ '-config', this.configFile ])).then(function(res) {
			if (res.code !== 0)
				throw new Error((res.stderr || res.stdout || '').trim() || _('Ошибка'));
			return res;
		});
	},

	saveBotConfig: function() {
		var self = this;
		var tasks = [];
		FIELDS.concat(ADVANCED).forEach(function(f) {
			var el = document.getElementById('hfbot-' + f.key);
			if (el && el.value !== el.getAttribute('data-initial'))
				tasks.push([ f.key, el.value.trim() ]);
		});
		var alerts = document.getElementById('hfbot-alerts');
		if (alerts && String(alerts.checked) !== alerts.getAttribute('data-initial'))
			tasks.push([ 'notify_failover_enabled', alerts.checked ? 'true' : 'false' ]);
		var interval = document.getElementById('hfbot-interval');
		if (interval && interval.value !== interval.getAttribute('data-initial'))
			tasks.push([ 'notify_failover_interval_seconds', interval.value.trim() ]);

		if (!tasks.length) {
			this.header.setResult(_('Изменений в настройках бота нет.'), 'info');
			return Promise.resolve();
		}

		this.header.setResult(_('Сохраняю настройки бота…'), 'info');
		var chain = Promise.resolve();
		tasks.forEach(function(kv) {
			chain = chain.then(function() {
				return self.exec([ '-mode', 'set-pending', '-key', kv[0], '-value', kv[1] ]);
			});
		});
		return chain.then(function() {
			return self.exec([ '-mode', 'apply-config' ]);
		}).then(function() {
			return callServiceRestart(BOT_SERVICE);
		}).then(function() {
			document.querySelectorAll('[data-initial]').forEach(function(el) {
				el.setAttribute('data-initial', el.type === 'checkbox' ? String(el.checked) : el.value);
			});
			self.header.setResult(_('Настройки бота сохранены, бот перезапущен.'), true);
			return self.refreshStatus();
		}).catch(function(err) {
			// Leave nothing half-written in the bot's draft.
			return self.exec([ '-mode', 'rollback-config' ]).catch(function() {}).then(function() {
				self.header.setResult(_('Не сохранено: ') + String(err.message || err), false);
			});
		});
	},

	field: function(f, value) {
		var input = E('input', {
			'id': 'hfbot-' + f.key,
			'class': 'cbi-input-text',
			'type': f.secret ? 'password' : (f.number ? 'number' : 'text'),
			'value': value,
			'data-initial': value,
			'placeholder': f.placeholder || f.def || '',
			'autocomplete': 'off',
			'spellcheck': 'false'
		});
		var control = input;
		if (f.secret) {
			control = E('div', { 'class': 'hf-field__row' }, [
				input,
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': function(ev) {
						ev.preventDefault();
						var show = input.type === 'password';
						input.type = show ? 'text' : 'password';
						ev.target.textContent = show ? _('Скрыть') : _('Показать');
					}
				}, _('Показать'))
			]);
		}
		return E('div', { 'class': 'hf-field' }, [
			E('label', { 'for': 'hfbot-' + f.key }, f.label),
			control,
			f.hint ? E('div', { 'class': 'hf-field__hint' }, f.hint) : ''
		]);
	},

	render: function(loaded) {
		var self = this;
		var cfg = {};
		try { cfg = JSON.parse(loaded[1] || '{}'); } catch (e) { cfg = {}; }
		var running = this.serviceState(loaded[2]);

		var value = function(f) {
			var v = cfg[f.key];
			if (Array.isArray(v))
				return v.join(', ');
			return v == null ? '' : String(v);
		};

		this.header = hfui.pageHeader({
			title: _('Telegram-бот'),
			pills: this.statusPills(running),
			actions: [
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(this, function() {
						return callServiceRestart(BOT_SERVICE).then(function() {
							self.header.setResult(_('Бот перезапущен.'), true);
							return self.refreshStatus();
						});
					})
				}, _('Перезапустить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, 'saveBotConfig')
				}, _('Сохранить настройки бота'))
			],
			hint: _('Бот присылает уведомления о переключениях и позволяет управлять роутером из Telegram. Настройки бота сохраняются кнопкой вверху, автозапуск службы ниже кнопкой «Применить».')
		});

		var alertsOn = !!cfg.notify_failover_enabled;
		var intervalVal = String(cfg.notify_failover_interval_seconds || 30);

		var botPanel = hfui.panel(_('Бот'), _('Подключение и доступ.'), FIELDS.map(function(f) {
			return self.field(f, value(f));
		}));

		var alertsPanel = hfui.panel(_('Уведомления'), _('Что бот присылает администраторам сам.'), [
			E('div', { 'class': 'hf-switch-row' }, [
				E('div', {}, [
					E('label', { 'for': 'hfbot-alerts' }, _('Сообщать о переключениях каналов')),
					E('div', { 'class': 'hf-field__hint' }, _('Например, когда основной VPN упал и трафик ушёл на резерв.'))
				]),
				E('div', { 'class': 'cbi-checkbox' }, [
					E('input', { 'id': 'hfbot-alerts', 'type': 'checkbox', 'checked': alertsOn ? '' : null, 'data-initial': String(alertsOn) }),
					E('label', { 'for': 'hfbot-alerts' })
				])
			]),
			E('div', { 'class': 'hf-field', 'style': 'margin-top:12px;' }, [
				E('label', { 'for': 'hfbot-interval' }, _('Не чаще чем раз в, сек')),
				E('input', { 'id': 'hfbot-interval', 'class': 'cbi-input-text', 'type': 'number', 'min': '10', 'value': intervalVal, 'data-initial': intervalVal }),
				E('div', { 'class': 'hf-field__hint' }, _('Защита от потока сообщений, если канал часто моргает. Не меньше 10.'))
			]),
			E('details', { 'class': 'hf-details' }, [
				E('summary', {}, _('Дополнительно')),
				E('div', {}, ADVANCED.map(function(f) { return self.field(f, value(f)); }))
			])
		]);

		var m = new form.Map('hybrid-failover-bot');
		var s = m.section(form.NamedSection, 'main', 'bot', _('Служба'));
		s.anonymous = true;
		s.tab('main', _('Запуск'));
		s.tab('paths', _('Файлы'));

		var o = s.taboption('main', form.Flag, 'enabled', _('Запускать бота вместе с роутером'));
		o.default = o.disabled;

		o = s.taboption('paths', form.Value, 'binary', _('Программа бота'));
		o.default = '/usr/bin/hybrid-failover-bot';
		o = s.taboption('paths', form.Value, 'config_path', _('Файл настроек'));
		o.default = '/etc/hybrid-failover-bot.json';
		o = s.taboption('paths', form.Value, 'log_path', _('Журнал'));
		o.default = '/var/log/hybrid-failover-bot.log';

		return m.render().then(function(mapEl) {
			var root = E('div', { 'class': 'hf-page hf-mon' }, [
				self.header,
				E('div', { 'class': 'hf-panels' }, [ botPanel, alertsPanel ]),
				mapEl
			]);
			hfui.injectStyles(root);
			return root;
		});
	}
});
