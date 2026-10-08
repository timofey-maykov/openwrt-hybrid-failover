'use strict';
'require view';
'require uci';
'require rpc';
'require ui';
'require hybrid-failover.hf-ui as hfui';

var PKG = 'hybrid-failover';

var callListRoutes = rpc.declare({ object: 'hybrid-failover', method: 'list_routes' });

var LIST_TITLES = {
	russia_inside: 'Russia inside', russia_outside: 'Russia outside', ukraine_inside: 'Ukraine inside',
	geoblock: 'Geoblock', block: 'Block', porn: 'Porn', news: 'News', anime: 'Anime', youtube: 'YouTube',
	hdrezka: 'HDRezka', tiktok: 'TikTok', google_ai: 'Google AI', google_play: 'Google Play', hodca: 'Hodca',
	discord: 'Discord', meta: 'Meta', twitter: 'Twitter', cloudflare: 'Cloudflare', cloudfront: 'Cloudfront',
	digitalocean: 'DigitalOcean', hetzner: 'Hetzner', ovh: 'OVH', telegram: 'Telegram', roblox: 'Roblox',
	netflix: 'Netflix', rocketleague: 'Rocket League'
};

var KIND_LABEL = {
	community: _('Сервис'),
	user: _('Из настроек секции'),
	local: _('Файлы из настроек секции'),
	user_list: _('Свой список')
};

var CSS = [
	'.hfs { max-width: 1280px; margin: 0 auto 28px; }',
	'.hfs-intro { font-size: 13px; opacity: .85; margin: 0 0 14px; line-height: 1.5; max-width: 900px; }',
	'.hfs-sec { margin-bottom: 22px; }',
	'.hfs-sec__head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; margin-bottom: 10px; }',
	'.hfs-sec__head h3 { margin: 0; font-size: 15px; font-weight: 700; flex: 1 1 auto; }',
	'.hfs-chans { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 10px; margin-bottom: 12px; }',
	'.hfs-chan { border: 1px solid var(--border-color, rgba(127,127,127,.3)); border-left-width: 4px; border-radius: 8px; padding: 10px 12px; background: var(--cbi-section-background-color, rgba(127,127,127,.04)); }',
	'.hfs-chan--up { border-left-color: #3cba54; }',
	'.hfs-chan--down { border-left-color: #e74c3c; }',
	'.hfs-chan--na { border-left-color: #f0ad4e; }',
	'.hfs-chan__top { display: flex; align-items: center; justify-content: space-between; gap: 6px; }',
	'.hfs-chan__name { font-weight: 700; font-size: 13px; word-break: break-word; }',
	'.hfs-chan__rename { appearance: none; border: 0; background: transparent; cursor: pointer; opacity: .55; font-size: 13px; padding: 0 2px; color: inherit; }',
	'.hfs-chan__rename:hover { opacity: 1; }',
	'.hfs-chan__meta { font-size: 12px; opacity: .75; margin-top: 2px; }',
	'.hfs-load { margin-top: 8px; }',
	'.hfs-load__bar { height: 6px; border-radius: 3px; background: rgba(127,127,127,.18); overflow: hidden; }',
	'.hfs-load__bar span { display: block; height: 100%; background: #2980b9; border-radius: 3px; transition: width .3s; }',
	'.hfs-load__txt { font-size: 11px; opacity: .7; margin-top: 3px; }',
	'.hfs-chips { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 6px; }',
	'.hfs-chip { font-size: 11px; padding: 1px 6px; border-radius: 4px; background: rgba(41,128,185,.14); }',
	'.hfs-table td, .hfs-table th { vertical-align: middle; }',
	'.hfs-table select { min-width: 150px; width: 100%; }',
	'.hfs-table tr.hfs-row--changed { background: rgba(240,173,78,.10) !important; }',
	'.hfs-w { letter-spacing: 1px; font-size: 10px; color: #2980b9; white-space: nowrap; }',
	'.hfs-w i { opacity: .25; font-style: normal; }',
	'.hfs-kind { font-size: 11px; opacity: .7; }',
	'.hfs-now { font-size: 12px; white-space: nowrap; }',
	'.hfs-now--warn { color: #b8860b; }',
	'.hfs-now--bad { color: #c0392b; }',
	'.hfs-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; align-items: center; }',
	'.hfs-actions .hfs-spacer { flex: 1; }',
	'.hfs-warn { padding: 8px 12px; border-radius: 6px; background: rgba(240,173,78,.12); border: 1px solid rgba(240,173,78,.45); font-size: 12px; margin-bottom: 10px; }',
	'.hfs-icon-btn { appearance: none; border: 0; background: transparent; cursor: pointer; color: inherit; opacity: .6; padding: 0 4px; }',
	'.hfs-icon-btn:hover { opacity: 1; }',
	'.hfs-form label { display: block; font-size: 12px; font-weight: 600; margin: 10px 0 4px; }',
	'.hfs-form input, .hfs-form textarea { width: 100%; box-sizing: border-box; }',
	'.hfs-form textarea { min-height: 110px; font-family: ui-monospace, monospace; font-size: 12px; }',
	'.hfs-form .hfs-help { font-size: 11px; opacity: .7; margin-top: 3px; }',
	'.hfs-dirty { font-size: 12px; color: #b8860b; }'
].join('\n');

function weightDots(w) {
	var n = Math.max(1, Math.min(5, Math.round(w / 2)));
	var out = [];
	for (var i = 0; i < 5; i++)
		out.push(i < n ? '●' : E('i', {}, '●'));
	return E('span', { 'class': 'hfs-w', 'title': _('Примерная доля трафика сервиса') }, out);
}

function fmtBytes(v) {
	var u = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ'], i = 0;
	while (v >= 1024 && i < u.length - 1) {
		v /= 1024;
		i++;
	}
	return (v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)) + ' ' + u[i];
}

function slug(s) {
	var map = { а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'e', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'h', ц: 'c', ч: 'ch', ш: 'sh', щ: 'sch', ы: 'y', э: 'e', ю: 'yu', я: 'ya' };
	s = String(s || '').toLowerCase().replace(/[а-яё]/g, function(c) { return map[c] || ''; });
	return s.replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 24);
}

function routeSectionName(section, key) {
	return ('lr_' + section + '_' + key).replace(/[^a-zA-Z0-9_]/g, '_');
}

return view.extend({
	handleSave: null,
	handleReset: null,

	load: function() {
		return Promise.all([
			uci.load(PKG),
			L.resolveDefault(callListRoutes(), null)
		]);
	},

	render: function(res) {
		var self = this;
		this.report = (hfui.unwrapData(res[1]) || {}).sections || (res[1] && res[1].sections) || [];
		this.state = {};
		this.root = E('div', { 'class': 'hfs' });
		hfui.injectStyles(this.root);
		this.root.appendChild(E('style', { 'type': 'text/css' }, CSS));
		this.root.classList.add('hf-page');
		this.root.appendChild(hfui.pageHeader({
			title: _('Сервисы и каналы'),
			hint: _('Привяжите списки сервисов к своим каналам, чтобы тяжелый трафик не шел через один туннель. Список без привязки идет через пул секции, как раньше: самый быстрый живой канал. «Делить по каналам» закрепляет каждый сайт за одним из живых каналов: быстрые получают больше сайтов, а канал намного медленнее лучшего новых сайтов не получает. Видео YouTube, Instagram и Telegram делится по серверам, один ролик целиком идет через один канал. Если выбранный канал упал, список временно уходит туда, что указано в колонке «Если канал упал».')
		}));
		if (!this.report.length) {
			this.root.appendChild(E('div', { 'class': 'hfs-warn' }, _('Нет секций vpn или proxy с каналами. Добавьте секцию на вкладке «Маршрутизация».')));
			return this.root;
		}
		this.report.forEach(function(sec) {
			self.root.appendChild(self.renderSection(sec));
		});
		return this.root;
	},

	channelLabel: function(sec, id) {
		for (var i = 0; i < sec.channels.length; i++)
			if (sec.channels[i].id === id)
				return sec.channels[i].name;
		return id;
	},

	/* Desired channel per list, starting from what UCI has. */
	stateFor: function(sec) {
		if (this.state[sec.name])
			return this.state[sec.name];
		var st = { lists: {}, onDown: {} };
		sec.lists.forEach(function(l) {
			st.lists[l.key] = l.channel || 'auto';
		});
		(sec.bindings || []).forEach(function(b) {
			(b.lists || []).forEach(function(k) {
				if (st.onDown[k] == null)
					st.onDown[k] = b.on_down || 'pool';
			});
		});
		st.initial = JSON.stringify(st);
		this.state[sec.name] = st;
		return st;
	},

	renderSection: function(sec) {
		var self = this;
		var st = this.stateFor(sec);
		var box = E('div', { 'class': 'hfs-sec hf-ent-card' });

		var warns = (sec.warnings || []).map(function(w) {
			var p = w.split(':');
			if (p[0] === 'missing_channel')
				return _('Привязка «%s» указывает на канал, которого больше нет. Ее списки сейчас идут через пул.').format(p[1]);
			if (p[0] === 'unknown_list')
				return _('В привязке «%s» есть список «%s», которого нет в секции.').format(p[1], p[2]);
			if (p[0] === 'single_channel')
				return _('В секции один канал: привязки к каналу не разгрузят его. Добавьте ссылки в urltest или резервные каналы.');
			return w;
		});

		var dirty = E('span', { 'class': 'hfs-dirty', 'style': 'display:none' }, _('Есть несохраненные изменения'));
		var table = E('table', { 'class': 'table hf-mon-table hfs-table' });
		var chansEl = E('div', { 'class': 'hfs-chans' });

		var redraw = function() {
			self.renderChannels(sec, chansEl);
			self.renderTable(sec, table, function() {
				dirty.style.display = JSON.stringify({ lists: st.lists, onDown: st.onDown }) !== JSON.stringify({ lists: JSON.parse(st.initial).lists, onDown: JSON.parse(st.initial).onDown }) ? '' : 'none';
				self.renderChannels(sec, chansEl);
			});
		};
		redraw();

		box.appendChild(E('div', { 'class': 'hfs-sec__head' }, [
			E('h3', {}, [_('Секция'), ' ', E('span', { 'class': 'hf-mon-tag' }, sec.name)]),
			E('span', { 'class': 'hf-mon-chip' }, sec.type)
		]));
		warns.forEach(function(w) { box.appendChild(E('div', { 'class': 'hfs-warn' }, w)); });
		box.appendChild(chansEl);
		box.appendChild(E('div', { 'class': 'hf-mon-table-wrap' }, table));
		box.appendChild(E('div', { 'class': 'hfs-actions' }, [
			E('button', {
				'class': 'btn cbi-button',
				'title': _('Тяжелые сервисы раскладываются по разным живым каналам, быстрые каналы получают больше'),
				'click': function() {
					var sugg = sec.suggest || {};
					if (!Object.keys(sugg).length) {
						ui.addNotification(null, E('p', {}, _('Нужно хотя бы два живых канала.')), 'warning');
						return;
					}
					Object.keys(sugg).forEach(function(k) {
						st.lists[k] = sugg[k];
						if (!st.onDown[k])
							st.onDown[k] = 'pool';
					});
					redraw();
					dirty.style.display = '';
				}
			}, _('Распределить автоматически')),
			E('button', {
				'class': 'btn cbi-button',
				'click': function() {
					Object.keys(st.lists).forEach(function(k) { st.lists[k] = 'auto'; });
					redraw();
					dirty.style.display = '';
				}
			}, _('Все через пул')),
			E('button', {
				'class': 'btn cbi-button cbi-button-add',
				'click': function() { self.editUserList(sec, null); }
			}, _('+ Свой список')),
			E('span', { 'class': 'hfs-spacer' }),
			dirty,
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(this, function() { return self.saveSection(sec); })
			}, _('Сохранить и применить'))
		]));
		return box;
	},

	renderChannels: function(sec, el) {
		var self = this;
		var st = this.stateFor(sec);
		var total = 0, byCh = {};
		sec.lists.forEach(function(l) {
			total += l.weight;
			var ch = st.lists[l.key] || 'auto';
			(byCh[ch] = byCh[ch] || { w: 0, lists: [] }).w += l.weight;
			byCh[ch].lists.push(l);
		});
		while (el.firstChild)
			el.removeChild(el.firstChild);
		sec.channels.forEach(function(c) {
			var state = c.up == null ? 'na' : (c.up ? 'up' : 'down');
			var mine = byCh[c.id] || { w: 0, lists: [] };
			var pct = total ? Math.round(mine.w / total * 100) : 0;
			el.appendChild(E('div', { 'class': 'hfs-chan hfs-chan--' + state }, [
				E('div', { 'class': 'hfs-chan__top' }, [
					E('span', { 'class': 'hfs-chan__name' }, c.name),
					E('button', {
						'class': 'hfs-chan__rename', 'title': _('Переименовать канал'),
						'click': function() { self.renameChannel(sec, c); }
					}, '✎')
				]),
				E('div', { 'class': 'hfs-chan__meta' }, [
					c.primary ? _('VPN-интерфейс') : (c.host || ''),
					' · ',
					state === 'up' ? (c.delay_ms ? c.delay_ms + ' ' + _('мс') : _('работает')) : (state === 'down' ? _('недоступен') : _('нет проверки')),
					c.active ? ' · ' + c.active + ' ' + _('соед.') : '',
					(c.rx || c.tx) ? ' · ↓' + fmtBytes(c.rx) + ' ↑' + fmtBytes(c.tx) : ''
				]),
				E('div', { 'class': 'hfs-load' }, [
					E('div', { 'class': 'hfs-load__bar' }, E('span', { 'style': 'width:' + pct + '%' })),
					E('div', { 'class': 'hfs-load__txt' }, mine.lists.length ?
						_('Закреплено %d%% ожидаемой нагрузки').format(pct) : _('Списки не закреплены, работает в пуле'))
				]),
				E('div', { 'class': 'hfs-chips' }, mine.lists.map(function(l) {
					return E('span', { 'class': 'hfs-chip' }, self.listTitle(l));
				}))
			]));
		});
	},

	listTitle: function(l) {
		if (l.kind === 'community')
			return LIST_TITLES[l.key] || l.key;
		if (l.kind === 'user')
			return _('Свои домены и подсети');
		if (l.kind === 'local')
			return _('Локальные списки');
		return l.title || l.key.replace(/^user:/, '');
	},

	nowText: function(sec, l) {
		var b = null;
		(sec.bindings || []).forEach(function(x) {
			if ((x.lists || []).indexOf(l.key) >= 0 && !b)
				b = x;
		});
		if (!b || l.channel === 'auto')
			return E('span', { 'class': 'hfs-now' }, _('пул секции'));
		if (b.missing)
			return E('span', { 'class': 'hfs-now hfs-now--warn' }, _('пул: канала нет'));
		switch (b.via) {
		case 'channel':
			return E('span', { 'class': 'hfs-now' }, '✓ ' + this.channelLabel(sec, b.channel));
		case 'pool':
			return E('span', { 'class': 'hfs-now hfs-now--warn' }, _('канал упал, идет через пул'));
		case 'direct':
			return E('span', { 'class': 'hfs-now' + (b.channel === 'direct' ? '' : ' hfs-now--warn') }, b.channel === 'direct' ? _('напрямую') : _('канал упал, идет напрямую'));
		case 'balance':
			return E('span', { 'class': 'hfs-now' }, _('по быстрым каналам'));
		case 'block':
			return E('span', { 'class': 'hfs-now hfs-now--bad' }, _('заблокировано'));
		}
		return E('span', { 'class': 'hfs-now' }, _('после применения'));
	},

	renderTable: function(sec, table, onChange) {
		var self = this;
		var st = this.stateFor(sec);
		var init = JSON.parse(st.initial);
		while (table.firstChild)
			table.removeChild(table.firstChild);
		table.appendChild(E('thead', {}, E('tr', {}, [
			E('th', {}, _('Список')),
			E('th', {}, _('Нагрузка')),
			E('th', {}, _('Канал')),
			E('th', {}, _('Если канал упал')),
			E('th', {}, _('Сейчас')),
			E('th', {}, '')
		])));
		var body = E('tbody');
		if (!sec.lists.length)
			body.appendChild(E('tr', {}, E('td', { 'colspan': 6, 'class': 'hf-mon-empty' },
				_('В секции нет списков. Включите сервисы на вкладке «Маршрутизация» или добавьте свой список.'))));
		sec.lists.forEach(function(l) {
			var cur = st.lists[l.key] || 'auto';
			var opts = [E('option', { 'value': 'auto' }, _('Пул (самый быстрый)'))];
			sec.channels.forEach(function(c) {
				opts.push(E('option', { 'value': c.id }, c.name + (c.up === false ? ' ' + _('(недоступен)') : '')));
			});
			opts.push(E('option', { 'value': 'balance' }, _('Делить по каналам')));
			opts.push(E('option', { 'value': 'direct' }, _('Напрямую, без туннеля')));
			opts.push(E('option', { 'value': 'block' }, _('Блокировать')));
			if (cur !== 'auto' && !sec.channels.some(function(c) { return c.id === cur; }) && ['balance', 'direct', 'block'].indexOf(cur) < 0)
				opts.push(E('option', { 'value': cur }, _('Удаленный канал %s').format(cur)));
			var sel = E('select', { 'class': 'cbi-input-select' }, opts);
			sel.value = cur;
			var bound = sec.channels.some(function(c) { return c.id === cur; }) || (cur !== 'auto' && ['balance', 'direct', 'block'].indexOf(cur) < 0);
			var down = E('select', { 'class': 'cbi-input-select', 'disabled': bound ? null : '' }, [
				E('option', { 'value': 'pool' }, _('В пул секции')),
				E('option', { 'value': 'direct' }, _('Напрямую')),
				E('option', { 'value': 'block' }, _('Не пускать'))
			]);
			down.value = st.onDown[l.key] || 'pool';
			sel.addEventListener('change', function() {
				st.lists[l.key] = sel.value;
				self.renderTable(sec, table, onChange);
				onChange();
			});
			down.addEventListener('change', function() {
				st.onDown[l.key] = down.value;
				self.renderTable(sec, table, onChange);
				onChange();
			});
			var changed = (init.lists[l.key] || 'auto') !== cur || (bound && (init.onDown[l.key] || 'pool') !== down.value);
			var tools = '';
			if (l.kind === 'user_list')
				tools = E('span', {}, [
					E('button', { 'class': 'hfs-icon-btn', 'title': _('Изменить'), 'click': function() { self.editUserList(sec, l); } }, '✎'),
					E('button', { 'class': 'hfs-icon-btn', 'title': _('Удалить'), 'click': function() { self.deleteUserList(sec, l); } }, '✕')
				]);
			body.appendChild(E('tr', { 'class': changed ? 'hfs-row--changed' : '' }, [
				E('td', {}, [E('div', { 'style': 'font-weight:600' }, self.listTitle(l)), E('div', { 'class': 'hfs-kind' }, KIND_LABEL[l.kind] || l.kind)]),
				E('td', {}, weightDots(l.weight)),
				E('td', {}, sel),
				E('td', {}, down),
				E('td', {}, changed ? E('span', { 'class': 'hfs-now hfs-now--warn' }, _('после применения')) : self.nowText(sec, l)),
				E('td', { 'style': 'text-align:right;white-space:nowrap' }, tools)
			]));
		});
		table.appendChild(body);
	},

	/* One list_route per bound list; lists in the pool have none. Sections
	 * are updated in place: removing and re-adding one name in a single save
	 * is applied in the wrong order by LuCI. */
	writeRoutes: function(sec) {
		var st = this.stateFor(sec);
		var want = {};
		sec.lists.forEach(function(l) {
			var ch = st.lists[l.key] || 'auto';
			if (ch !== 'auto')
				want[routeSectionName(sec.name, l.key)] = { key: l.key, ch: ch, down: st.onDown[l.key] || 'pool' };
		});
		var have = {};
		uci.sections(PKG, 'list_route', function(s) {
			if (s.section !== sec.name)
				return;
			if (want[s['.name']])
				have[s['.name']] = true;
			else
				uci.remove(PKG, s['.name']);
		});
		Object.keys(want).forEach(function(name) {
			var w = want[name];
			if (!have[name])
				uci.add(PKG, 'list_route', name);
			uci.set(PKG, name, 'section', sec.name);
			uci.set(PKG, name, 'lists', [w.key]);
			uci.set(PKG, name, 'channel', w.ch);
			uci.set(PKG, name, 'on_down', w.down);
			uci.unset(PKG, name, 'enabled');
		});
	},

	saveSection: function(sec) {
		this.writeRoutes(sec);
		return uci.save().then(function() {
			return ui.changes.apply(true);
		});
	},

	renameChannel: function(sec, c) {
		var input = E('input', { 'class': 'cbi-input-text', 'value': c.name, 'maxlength': 40 });
		var self = this;
		hfui.showModal(_('Название канала'), [
			E('div', { 'class': 'hfs-form' }, [
				E('label', {}, _('Как показывать канал в интерфейсе и в боте')),
				input,
				E('div', { 'class': 'hfs-help' }, _('Название привязано к серверу и ключу, а не к позиции в списке, так что переживет перестановку ссылок.'))
			])
		], function() {
			var names = (uci.get(PKG, sec.name, 'channel_names') || []).filter(function(v) {
				return v.split('=')[0] !== c.id;
			});
			var v = input.value.trim().replace(/=/g, '-');
			if (v)
				names.push(c.id + '=' + v);
			uci.set(PKG, sec.name, 'channel_names', names.length ? names : null);
			self.writeRoutes(sec);
			return uci.save().then(function() { return ui.changes.apply(true); });
		});
	},

	editUserList: function(sec, l) {
		var self = this;
		var existing = l ? l.key.replace(/^user:/, '') : null;
		var title = E('input', { 'class': 'cbi-input-text', 'placeholder': _('Например: Работа'), 'value': existing ? (uci.get(PKG, existing, 'title') || '') : '' });
		var domains = E('textarea', { 'class': 'cbi-input-textarea', 'placeholder': 'jira.example.com\ngitlab.example.com' },
			existing ? (uci.get(PKG, existing, 'domains_text') || '') : '');
		var subnets = E('textarea', { 'class': 'cbi-input-textarea', 'placeholder': '10.20.0.0/16\n203.0.113.7' },
			existing ? (uci.get(PKG, existing, 'subnets_text') || '') : '');
		var chOpts = [E('option', { 'value': 'auto' }, _('Пул (самый быстрый)'))].concat(sec.channels.map(function(c) {
			return E('option', { 'value': c.id }, c.name);
		}), [E('option', { 'value': 'balance' }, _('Делить по каналам')), E('option', { 'value': 'direct' }, _('Напрямую'))]);
		var ch = E('select', { 'class': 'cbi-input-select' }, chOpts);
		ch.value = l ? (this.stateFor(sec).lists[l.key] || 'auto') : 'auto';

		hfui.showModal(existing ? _('Изменить список') : _('Свой список'), [
			E('div', { 'class': 'hfs-form' }, [
				E('label', {}, _('Название')), title,
				E('label', {}, _('Домены')), domains,
				E('div', { 'class': 'hfs-help' }, _('По одному на строку или через пробел. Поддомены входят: example.com покрывает www.example.com.')),
				E('label', {}, _('Подсети и IP')), subnets,
				E('div', { 'class': 'hfs-help' }, _('CIDR или отдельные адреса IPv4.')),
				E('label', {}, _('Канал')), ch
			])
		], function() {
			var t = title.value.trim();
			if (!t || (!domains.value.trim() && !subnets.value.trim())) {
				ui.addNotification(null, E('p', {}, _('Нужны название и хотя бы один домен или подсеть.')), 'warning');
				return Promise.reject();
			}
			var bad = subnets.value.split(/[\s,;]+/).filter(function(x) {
				return x && !/^\d{1,3}(\.\d{1,3}){3}(\/\d{1,2})?$/.test(x);
			});
			if (bad.length) {
				ui.addNotification(null, E('p', {}, _('Не похоже на IPv4 или CIDR: %s').format(bad.join(', '))), 'warning');
				return Promise.reject();
			}
			var name = existing;
			if (!name) {
				name = 'ul_' + (slug(t) || 'list');
				var base = name, n = 2;
				while (uci.get(PKG, name))
					name = base + '_' + n++;
				uci.add(PKG, 'user_list', name);
			}
			uci.set(PKG, name, 'section', sec.name);
			uci.set(PKG, name, 'title', t);
			uci.set(PKG, name, 'domains_text', domains.value.trim() || null);
			uci.set(PKG, name, 'subnets_text', subnets.value.trim() || null);
			var key = 'user:' + name;
			var st = self.stateFor(sec);
			if (!sec.lists.some(function(x) { return x.key === key; }))
				sec.lists.push({ key: key, title: t, kind: 'user_list', weight: 2, channel: 'auto' });
			st.lists[key] = ch.value;
			self.writeRoutes(sec);
			return uci.save().then(function() { return ui.changes.apply(true); });
		}, { wide: true });
	},

	deleteUserList: function(sec, l) {
		var self = this;
		var name = l.key.replace(/^user:/, '');
		hfui.showModal(_('Удалить список'), [E('p', {}, _('Удалить список «%s» и его привязку?').format(self.listTitle(l)))], function() {
			uci.remove(PKG, name);
			sec.lists = sec.lists.filter(function(x) { return x.key !== l.key; });
			delete self.stateFor(sec).lists[l.key];
			self.writeRoutes(sec);
			return uci.save().then(function() { return ui.changes.apply(true); });
		});
	},

	handleSaveApply: function() {
		var self = this;
		this.report.forEach(function(sec) { self.writeRoutes(sec); });
		return uci.save().then(function() { return ui.changes.apply(true); });
	}
});
