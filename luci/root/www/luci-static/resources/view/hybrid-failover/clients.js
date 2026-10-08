'use strict';
'require view';
'require form';
'require uci';
'require ui';
'require hybrid-failover.hf-ui as hfui';

// Per-device rules (client_rule). The modes map to the core like this:
// include and full_route mark all traffic of the device for the engine,
// full_route also sends it through one section; exclude keeps the device's
// DNS out of fake-IP so its traffic never reaches the tunnel; global_exclude
// routes whatever the engine gets from the device straight out.
var MODES = [
	[ 'full_route', _('Весь трафик через туннель') ],
	[ 'include', _('Весь трафик через Hybrid Failover, дальше по спискам') ],
	[ 'exclude', _('Не трогать (DNS и трафик мимо)') ],
	[ 'global_exclude', _('Всегда напрямую') ]
];

var MODE_SHORT = {
	full_route: _('весь трафик через туннель'),
	include: _('по спискам, включая обращения по IP'),
	exclude: _('не трогать'),
	global_exclude: _('всегда напрямую')
};

var CLIENTS_CSS = [
	'.hf-modes { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; margin: 0 0 18px; }',
	'.hf-modes > div { padding: 10px 12px; border-radius: 10px; background: var(--hf-surface); border: 1px solid var(--hf-line); font-size: 12.5px; line-height: 1.45; color: var(--hf-muted); }',
	'.hf-modes strong { display: block; color: var(--nb-text, inherit); margin-bottom: 2px; font-size: 13px; }',
	'.hf-dev { display: flex; flex-direction: column; } .hf-dev small { color: var(--hf-muted); font-size: 11.5px; }',
	'.hf-picker-list { display: flex; flex-direction: column; gap: 6px; max-height: 50vh; overflow: auto; margin-top: 10px; }',
	'.hf-picker-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 8px 10px; border: 1px solid var(--hf-line); border-radius: 8px; }',
	'.hf-picker-row .hf-dev { min-width: 0; } .hf-picker-row strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }'
].join('\n');

function normalizeLeases(res) {
	var d = hfui.unwrapData(res) || res;
	var leases = (d && d.leases) ? d.leases : [];
	if (!leases.length && d && d.dhcp_leases)
		leases = d.dhcp_leases;
	if (!leases.length && d && d.dhcp && d.dhcp.leases)
		leases = d.dhcp.leases;
	if (!leases.length && d && d['dhcp.leases'])
		leases = d['dhcp.leases'];
	return leases.map(function(l) {
		return {
			ip: l.ipaddr || l.ip || l.address || '',
			host: l.hostname || '',
			mac: l.mac || l.macaddr || ''
		};
	}).filter(function(l) { return l.ip; });
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('hybrid-failover'),
			L.resolveDefault(hfui.rpc.dhcpLeases(), null),
			L.resolveDefault(hfui.rpc.listClients(), null),
			L.resolveDefault(hfui.rpc.status(), null)
		]);
	},

	hostFor: function(ip) {
		ip = String(ip || '').replace(/\/32$/, '');
		for (var i = 0; i < this.leases.length; i++)
			if (this.leases[i].ip === ip)
				return this.leases[i].host || this.leases[i].mac;
		return '';
	},

	deviceCell: function(ip) {
		var host = this.hostFor(ip);
		return E('span', { 'class': 'hf-dev' }, [
			E('strong', {}, host || ip),
			host ? E('small', {}, ip) : ''
		]);
	},

	renderEffective: function(res, statusRes) {
		var self = this;
		var d = hfui.unwrapData(res) || {};
		var rules = Array.isArray(d.rules) ? d.rules : (Array.isArray(d.clients) ? d.clients : []);
		var st = statusRes ? hfui.unwrapData(statusRes) : null;
		var running = st ? hfui.proxyRunning(st) : null;

		if (!rules.length) {
			return E('div', { 'class': 'hf-empty' }, running === false ?
				_('Движок не запущен. Правила начнут действовать после запуска.') :
				_('Отдельных правил нет: все устройства идут по общим правилам секций.'));
		}
		var legacy = rules.some(function(c) { return /^legacy-/.test(c.source || c.name || ''); });
		return E('div', {}, [
			hfui.wrapTable(E('table', { 'class': 'hf-mon-table' }, [
				E('thead', {}, E('tr', {}, [
					E('th', {}, _('Устройство')),
					E('th', {}, _('Что делаем')),
					E('th', {}, _('Секция'))
				])),
				E('tbody', {}, rules.map(function(c) {
					return E('tr', {}, [
						E('td', {}, self.deviceCell(c.ip || c.ipaddr)),
						E('td', {}, MODE_SHORT[c.mode] || c.mode || '-'),
						E('td', {}, c.section || '-')
					]);
				}))
			])),
			legacy ? E('p', { 'class': 'hf-panel__sub' },
				_('Часть правил взята из старых настроек секций («Устройства в обход», «Устройства целиком через туннель»). Как только здесь появится хоть одно правило, старые перестанут учитываться.')) : ''
		]);
	},

	addFromNetwork: function() {
		var self = this;
		var used = {};
		uci.sections('hybrid-failover', 'client_rule').forEach(function(s) { used[s.ip] = true; });
		var leases = this.leases.filter(function(l) { return !used[l.ip]; });
		if (!leases.length) {
			ui.addNotification(null, E('p', {}, _('Нет подключённых устройств без правила. Можно добавить строку вручную в таблице ниже.')), 'info');
			return Promise.resolve();
		}
		var modeSel = E('select', { 'class': 'cbi-input-select' }, MODES.map(function(m) {
			return E('option', { 'value': m[0] }, m[1]);
		}));
		var sections = uci.sections('hybrid-failover', 'section').map(function(s) { return s['.name']; });
		var secSel = E('select', { 'class': 'cbi-input-select' }, sections.map(function(n) {
			return E('option', { 'value': n }, n);
		}));
		var secField = E('div', { 'class': 'hf-field' }, [ E('label', {}, _('Через какую секцию')), secSel ]);
		modeSel.addEventListener('change', function() {
			secField.style.display = modeSel.value === 'full_route' ? '' : 'none';
		});

		var list = E('div', { 'class': 'hf-picker-list' }, leases.map(function(l) {
			return E('div', { 'class': 'hf-picker-row' }, [
				E('span', { 'class': 'hf-dev' }, [
					E('strong', {}, l.host || l.ip),
					E('small', {}, l.ip + (l.mac ? ' · ' + l.mac : ''))
				]),
				E('button', {
					'class': 'btn cbi-button cbi-button-add',
					'click': function() {
						var sid = uci.add('hybrid-failover', 'client_rule');
						uci.set('hybrid-failover', sid, 'ip', l.ip);
						uci.set('hybrid-failover', sid, 'mode', modeSel.value);
						if (modeSel.value === 'full_route' && secSel.value)
							uci.set('hybrid-failover', sid, 'section', secSel.value);
						document.querySelectorAll('.hf-mon-modal-backdrop').forEach(function(el) { el.parentNode.removeChild(el); });
						return self.map.load().then(function() { return self.map.reset(); }).then(function() {
							ui.addNotification(null, E('p', {}, _('Правило для %s добавлено. Нажмите «Применить» внизу страницы.').format(l.host || l.ip)), 'info');
						});
					}
				}, _('Добавить'))
			]);
		}));

		hfui.showModal(_('Добавить устройство из сети'), [
			E('div', { 'class': 'hf-page' }, [
				E('div', { 'class': 'hf-field' }, [ E('label', {}, _('Что делать с устройством')), modeSel ]),
				secField,
				list
			])
		], null, { wide: true });
		return Promise.resolve();
	},

	render: function(loaded) {
		var self = this;
		this.leases = normalizeLeases(loaded[1]);
		var effective = this.renderEffective(loaded[2], loaded[3]);
		var sections = uci.sections('hybrid-failover', 'section').map(function(s) { return s['.name']; });

		var m = new form.Map('hybrid-failover');
		this.map = m;

		var s = m.section(form.GridSection, 'client_rule', _('Правила'));
		s.anonymous = true;
		s.addremove = true;
		s.sortable = true;
		s.addbtntitle = _('Добавить строку');
		s.nodescriptions = true;

		var o = s.option(form.Value, 'ip', _('IP или подсеть'));
		o.datatype = 'or(ip4addr, cidr4)';
		o.rmempty = false;
		o.editable = true;
		o.placeholder = '192.168.1.50';

		o = s.option(form.DummyValue, '_host', _('Имя'));
		o.cfgvalue = function(sid) {
			return self.hostFor(uci.get('hybrid-failover', sid, 'ip')) || '-';
		};

		o = s.option(form.ListValue, 'mode', _('Что делать'));
		MODES.forEach(function(md) { o.value(md[0], md[1]); });
		o.default = 'full_route';
		o.editable = true;

		o = s.option(form.ListValue, 'section', _('Секция'));
		sections.forEach(function(n) { o.value(n, n); });
		o.depends('mode', 'full_route');
		o.editable = true;

		var header = hfui.pageHeader({
			title: _('Клиенты'),
			pills: [ hfui.pill(_('Правил: ') + uci.sections('hybrid-failover', 'client_rule').length, 'plain') ],
			actions: [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, 'addFromNetwork')
				}, _('Добавить устройство из сети'))
			],
			hint: _('Отдельные правила для устройств домашней сети. Например, приставку можно целиком пустить через туннель, а рабочий ноутбук оставить напрямую. После изменений нажмите «Применить» внизу страницы.')
		});

		return m.render().then(function(mapEl) {
			var root = E('div', { 'class': 'hf-page hf-mon' }, [
				E('style', { 'type': 'text/css' }, CLIENTS_CSS),
				header,
				E('div', { 'class': 'hf-modes' }, [
					E('div', {}, [ E('strong', {}, MODES[0][1]), _('Всё, что делает устройство, идёт через выбранную секцию. Подходит для приставок и телевизоров.') ]),
					E('div', {}, [ E('strong', {}, MODES[1][1]), _('Через туннель идут только сервисы из списков, но даже обращения напрямую по IP проходят проверку.') ]),
					E('div', {}, [ E('strong', {}, MODES[2][1]), _('Hybrid Failover не трогает устройство: его DNS не подменяется, трафик в туннель не попадает.') ]),
					E('div', {}, [ E('strong', {}, MODES[3][1]), _('Даже сервисы из списков идут с этого устройства напрямую, мимо туннеля.') ])
				]),
				mapEl,
				E('div', { 'class': 'hf-panels', 'style': 'margin-top:18px;' }, [
					hfui.panel(_('Что действует сейчас'), _('Так правила видит работающий движок. Несохранённые изменения здесь появятся после применения.'), [ effective ], { wide: true })
				])
			]);
			hfui.injectStyles(root);
			return root;
		});
	}
});
