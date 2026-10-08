'use strict';
'require view';
'require fs';
'require ui';
'require hybrid-failover.hf-charts as hfc';
'require hybrid-failover.hf-ui as hfui';

var FINE_FILE = '/var/run/hybrid-failover/channel-metrics.json';
var COARSE_FILE = '/var/run/hybrid-failover/channel-metrics-24h.json';
var FINE_SPAN = 600;

var RANGES = [
	{ id: '5m', label: _('5 мин'), sec: 300 },
	{ id: '10m', label: _('10 мин'), sec: 600 },
	{ id: '1h', label: _('1 час'), sec: 3600 },
	{ id: '6h', label: _('6 часов'), sec: 21600 },
	{ id: '24h', label: _('24 часа'), sec: 86400 }
];

var REFRESH = [
	{ id: 2, label: _('2 с') },
	{ id: 5, label: _('5 с') },
	{ id: 10, label: _('10 с') },
	{ id: 30, label: _('30 с') },
	{ id: 0, label: _('Выкл.') }
];

var KIND_LABEL = {
	awg2: 'AWG', vpn: 'VPN', hysteria2: 'Hysteria2', vless: 'VLESS', trojan: 'Trojan',
	shadowsocks: 'Shadowsocks', socks: 'SOCKS', pool: _('Пул')
};

var CSS = [
	'.hft { max-width: 1440px; margin: 0 auto 28px; }',
	'.hft-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 14px; margin: 0 0 14px; }',
	'.hft-bar h2.hf-head__title { margin: 0; flex: 1 1 auto; }',
	'.hft-bar.hf-head { gap: 8px 14px; padding: 14px 18px; }',
	'.hft-seg { display: inline-flex; border: 1px solid var(--border-color, rgba(127,127,127,.35)); border-radius: 6px; overflow: hidden; }',
	'.hft-seg button { appearance: none; border: 0; background: transparent; color: inherit; padding: 5px 10px; font-size: 12px; cursor: pointer; border-right: 1px solid var(--border-color, rgba(127,127,127,.25)); }',
	'.hft-seg button:last-child { border-right: 0; }',
	'.hft-seg button.on { background: var(--nb-accent, #3d71d9); color: var(--nb-on-accent, #fff); }',
	'.hft-ctl { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; }',
	'.hft-ctl select { min-width: 0; width: auto; padding: 3px 6px; height: auto; font-size: 12px; }',
	'.hft-live { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; opacity: .85; }',
	'.hft-live__dot { width: 8px; height: 8px; border-radius: 50%; background: #73BF69; box-shadow: 0 0 0 0 rgba(115,191,105,.6); animation: hftPulse 2s infinite; }',
	'.hft-live--paused .hft-live__dot { background: #8e8e9e; animation: none; }',
	'@keyframes hftPulse { 0% { box-shadow: 0 0 0 0 rgba(115,191,105,.55); } 70% { box-shadow: 0 0 0 7px rgba(115,191,105,0); } 100% { box-shadow: 0 0 0 0 rgba(115,191,105,0); } }',
	'.hft-zoom { font-size: 12px; padding: 3px 10px; border-radius: 999px; background: rgba(61,113,217,.15); color: #3d71d9; cursor: pointer; border: 0; }',
	'.hft-tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 10px; margin-bottom: 14px; }',
	'.hft-tile { position: relative; border-radius: 6px; border: 1px solid var(--hfc-border); background: var(--hfc-bg); color: var(--hfc-text); padding: 10px 12px 6px 14px; overflow: hidden; cursor: pointer; transition: box-shadow .15s; }',
	'.hft-tile:hover { box-shadow: 0 0 0 1px var(--hfc-accent); }',
	'.hft-tile--off { opacity: .45; }',
	'.hft-tile__stripe { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }',
	'.hft-tile__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }',
	'.hft-tile__name { font-size: 13px; font-weight: 700; line-height: 1.3; word-break: break-word; }',
	'.hft-tile__sub { font-size: 11px; color: var(--hfc-muted); margin-top: 1px; }',
	'.hft-pill { font-size: 10px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; padding: 2px 7px; border-radius: 3px; white-space: nowrap; }',
	'.hft-pill--up { background: rgba(115,191,105,.18); color: #73BF69; }',
	'.hft-pill--down { background: rgba(242,73,92,.18); color: #F2495C; }',
	'.hft-pill--na { background: rgba(142,142,158,.18); color: #8e8e9e; }',
	'.hft-tile__nums { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 2px 12px; margin: 8px 0 4px; }',
	'.hft-num__label { font-size: 10px; text-transform: uppercase; letter-spacing: .05em; color: var(--hfc-muted); }',
	'.hft-num__val { font-size: 17px; font-weight: 600; font-variant-numeric: tabular-nums; line-height: 1.25; white-space: nowrap; }',
	'.hft-num__val small { font-size: 11px; font-weight: 500; color: var(--hfc-muted); margin-left: 3px; }',
	'.hft-tile__foot { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 4px; font-size: 11px; color: var(--hfc-muted); }',
	'.hft-chip { padding: 1px 6px; border-radius: 3px; background: var(--hfc-hover); border: 1px solid var(--hfc-border); color: var(--hfc-text); }',
	'.hft-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }',
	'.hft-grid .hft-wide { grid-column: 1 / -1; }',
	'@media (max-width: 980px) { .hft-grid { grid-template-columns: 1fr; } }',
	'.hft-share { border-radius: 6px; border: 1px solid var(--hfc-border); background: var(--hfc-bg); color: var(--hfc-text); padding: 10px 12px; }',
	'.hft-share__title { font-size: 13px; font-weight: 600; margin: 0 0 8px; }',
	'.hft-share__bar { display: flex; height: 22px; border-radius: 4px; overflow: hidden; background: var(--hfc-hover); }',
	'.hft-share__bar span { display: block; height: 100%; transition: width .4s; }',
	'.hft-share__legend { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-top: 8px; font-size: 12px; }',
	'.hft-share__item { display: inline-flex; align-items: center; gap: 6px; }',
	'.hft-share__item b { font-variant-numeric: tabular-nums; }',
	'.hft-empty { padding: 28px; text-align: center; border: 1px dashed var(--border-color, rgba(127,127,127,.35)); border-radius: 8px; font-size: 13px; opacity: .85; line-height: 1.6; }',
	'.hft-hint { font-size: 11px; opacity: .65; margin: 10px 0 0; }'
].join('\n');

function bits(v) { return v == null ? null : v * 8; }

function splitUnit(text) {
	var i = text.lastIndexOf(' ');
	return i < 0 ? [text, ''] : [text.slice(0, i), text.slice(i + 1)];
}

function numCell(label, text) {
	var parts = splitUnit(text);
	return E('div', {}, [
		E('div', { 'class': 'hft-num__label' }, label),
		E('div', { 'class': 'hft-num__val' }, [parts[0], parts[1] ? E('small', {}, parts[1]) : ''])
	]);
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	range: 600,
	refresh: 2,
	paused: false,
	zoom: null,
	section: '',
	fine: null,
	coarse: null,
	coarseAt: 0,
	timer: null,

	load: function() {
		return Promise.all([
			this.readFile(FINE_FILE),
			this.readFile(COARSE_FILE)
		]);
	},

	readFile: function(path) {
		return L.resolveDefault(fs.read_direct(path, 'json'), null).then(function(v) {
			return v && v.format ? v : null;
		});
	},

	render: function(res) {
		var self = this;
		this.fine = res[0];
		this.coarse = res[1];
		this.coarseAt = Date.now();
		try {
			var saved = JSON.parse(localStorage.getItem('hf-tunnels') || '{}');
			if (saved.range)
				this.range = saved.range;
			if (saved.refresh != null)
				this.refresh = saved.refresh;
		}
		catch (e) {}

		hfc.injectCSS();
		var root = E('div', { 'class': 'hft hf-page' });
		hfui.injectStyles(root);
		root.appendChild(E('style', { 'type': 'text/css' }, CSS));
		hfc.applyTheme(root, hfc.theme());
		this.root = root;
		this.group = new hfc.Group();
		this.group.onChange = function() { self.redrawTiles(); };

		this.liveEl = E('span', { 'class': 'hft-live' }, [E('span', { 'class': 'hft-live__dot' }), E('span', {}, '')]);
		this.zoomBtn = E('button', {
			'class': 'hft-zoom', 'style': 'display:none',
			'click': function() { self.resetZoom(); }
		}, _('Сбросить масштаб ✕'));
		this.sectionSel = E('select', {
			'class': 'cbi-input-select',
			'change': function(ev) {
				self.section = ev.target.value;
				self.redraw(true);
			}
		});
		var rangeSeg = E('div', { 'class': 'hft-seg' });
		RANGES.forEach(function(r) {
			rangeSeg.appendChild(E('button', {
				'class': r.sec === self.range ? 'on' : '',
				'click': function(ev) {
					self.range = r.sec;
					self.zoom = null;
					Array.prototype.forEach.call(rangeSeg.children, function(b) { b.classList.remove('on'); });
					ev.target.classList.add('on');
					self.save();
					self.tick(true);
				}
			}, r.label));
		});
		var refreshSel = E('select', {
			'class': 'cbi-input-select',
			'change': function(ev) {
				self.refresh = +ev.target.value;
				self.save();
				self.schedule();
			}
		}, REFRESH.map(function(r) {
			return E('option', { 'value': r.id, 'selected': r.id === self.refresh ? '' : null }, r.label);
		}));
		this.pauseBtn = E('button', {
			'class': 'btn cbi-button',
			'click': function() {
				self.paused = !self.paused;
				self.pauseBtn.textContent = self.paused ? _('Продолжить') : _('Пауза');
				self.updateLive();
				if (!self.paused)
					self.tick(true);
			}
		}, _('Пауза'));

		root.appendChild(E('div', { 'class': 'hft-bar hf-head' }, [
			E('h2', { 'class': 'hf-head__title' }, _('Графики')),
			this.liveEl,
			this.zoomBtn,
			E('label', { 'class': 'hft-ctl' }, [_('Секция'), this.sectionSel]),
			rangeSeg,
			E('label', { 'class': 'hft-ctl' }, [_('Обновление'), refreshSel]),
			this.pauseBtn
		]));

		this.tilesEl = E('div', { 'class': 'hft-tiles' });
		root.appendChild(this.tilesEl);
		this.emptyEl = E('div', { 'class': 'hft-empty', 'style': 'display:none' });
		root.appendChild(this.emptyEl);

		var zoomFn = function(a, b) { self.setZoom(a, b); };
		var resetFn = function() { self.resetZoom(); };
		var mk = function(o) {
			o.group = self.group;
			o.onZoom = zoomFn;
			o.onReset = resetFn;
			return new hfc.TimeSeries(o);
		};
		this.charts = {
			rx: mk({ title: _('Входящий трафик'), desc: _('из туннеля к клиентам'), unit: 'bps', height: 220, stackToggle: true }),
			tx: mk({ title: _('Исходящий трафик'), desc: _('от клиентов в туннель'), unit: 'bps', height: 220, stackToggle: true }),
			active: mk({ title: _('Открытые соединения'), unit: 'count', height: 190, fill: true }),
			newc: mk({ title: _('Новые соединения'), desc: _('в секунду'), unit: 'rate', height: 190, fill: false }),
			delay: mk({
				title: _('Задержка проверки'), desc: _('URL-тест канала, красным отмечены отказы'), unit: 'ms', height: 200, fill: false,
				thresholds: [{ value: 300, color: 'rgb(250,222,42)' }, { value: 1000, color: 'rgb(242,73,92)' }],
				legendCols: ['last', 'avg', 'min', 'max']
			})
		};
		this.shareEl = E('div', { 'class': 'hft-share' });
		var wide = function(el) { el.classList.add('hft-wide'); return el; };
		this.gridEl = E('div', { 'class': 'hft-grid' }, [
			this.charts.rx.el, this.charts.tx.el,
			wide(this.shareEl),
			this.charts.active.el, this.charts.newc.el,
			wide(this.charts.delay.el)
		]);
		root.appendChild(this.gridEl);
		root.appendChild(E('p', { 'class': 'hft-hint' },
			_('Выделите участок графика мышью, чтобы приблизить, двойной клик возвращает масштаб. Клик по каналу в легенде или по плитке оставляет только его, Ctrl/Cmd+клик скрывает или показывает.')));

		document.addEventListener('visibilitychange', function() {
			if (!document.hidden)
				self.tick(true);
		});
		requestAnimationFrame(function() { self.redraw(true); });
		this.schedule();
		return root;
	},

	save: function() {
		try {
			localStorage.setItem('hf-tunnels', JSON.stringify({ range: this.range, refresh: this.refresh }));
		}
		catch (e) {}
	},

	updateLive: function() {
		var paused = this.paused || this.zoom || !this.refresh;
		this.liveEl.classList.toggle('hft-live--paused', !!paused);
		var label = this.liveEl.lastChild;
		var upd = this.fine && this.fine.updated ? hfc.fmtFull(this.fine.updated).slice(11) : '';
		label.textContent = (paused ? _('Пауза') : _('Онлайн')) + (upd ? ' · ' + upd : '');
	},

	schedule: function() {
		var self = this;
		if (this.timer)
			clearTimeout(this.timer);
		this.timer = null;
		this.updateLive();
		if (!this.refresh)
			return;
		this.timer = setTimeout(function() { self.tick(false); }, this.refresh * 1000);
	},

	tick: function(force) {
		var self = this;
		if (!document.body.contains(this.root))
			return;
		if ((this.paused || this.zoom || document.hidden) && !force) {
			this.schedule();
			return;
		}
		var needCoarse = this.span() > FINE_SPAN && (force || Date.now() - this.coarseAt > 30000);
		return Promise.all([
			this.readFile(FINE_FILE),
			needCoarse ? this.readFile(COARSE_FILE) : Promise.resolve(this.coarse)
		]).then(function(r) {
			self.fine = r[0];
			if (needCoarse) {
				self.coarse = r[1];
				self.coarseAt = Date.now();
			}
			self.redraw(false);
		}).finally(function() { self.schedule(); });
	},

	span: function() {
		return this.zoom ? this.zoom.to - this.zoom.from : this.range;
	},

	setZoom: function(a, b) {
		if (b - a < 10)
			return;
		this.zoom = { from: a, to: b };
		this.zoomBtn.style.display = '';
		this.redraw(false);
		this.schedule();
	},

	resetZoom: function() {
		this.zoom = null;
		this.zoomBtn.style.display = 'none';
		this.tick(true);
	},

	/* Channel list, colours fixed by order so a channel keeps its colour. */
	channels: function() {
		var src = (this.fine && this.fine.channels) || (this.coarse && this.coarse.channels) || [];
		var sections = {};
		src.forEach(function(c) { sections[c.section] = true; });
		var many = Object.keys(sections).length > 1;
		var self = this;
		return src.map(function(c, i) {
			return Object.assign({}, c, {
				color: hfc.PALETTE[i % hfc.PALETTE.length],
				label: many ? c.name + ' · ' + c.section : c.name
			});
		}).filter(function(c) { return !self.section || c.section === self.section; });
	},

	/* Points of the visible window: coarse minutes up to where the fine ring
	 * starts, then the fine 2 s samples. */
	window: function() {
		var fine = this.fine, coarse = this.coarse;
		var now = (fine && fine.updated) || Math.floor(Date.now() / 1000);
		var from = this.zoom ? this.zoom.from : now - this.range;
		var to = this.zoom ? this.zoom.to : now;
		var t = [], idx = [];
		var fineStart = fine && fine.t.length ? fine.t[0] : Infinity;
		var useCoarse = coarse && coarse.t && from < fineStart && (to - from) > FINE_SPAN / 2;
		if (useCoarse)
			for (var i = 0; i < coarse.t.length; i++)
				if (coarse.t[i] >= from - 60 && coarse.t[i] < fineStart && coarse.t[i] <= to)
					idx.push(['c', i]), t.push(coarse.t[i]);
		if (fine)
			for (var j = 0; j < fine.t.length; j++)
				if (fine.t[j] >= from - 2 && fine.t[j] <= to + 2)
					idx.push(['f', j]), t.push(fine.t[j]);
		var step = (to - from) > FINE_SPAN ? 60 : (fine ? fine.step : 2);
		return { t: t, idx: idx, from: from, to: to, step: step };
	},

	pick: function(win, key, field, map) {
		var fine = this.fine, coarse = this.coarse;
		var fs_ = fine && fine.series && fine.series[key];
		var cs = coarse && coarse.series && coarse.series[key];
		return win.idx.map(function(p) {
			var s = p[0] === 'f' ? fs_ : cs;
			if (!s || !s[field])
				return null;
			var v = s[field][p[1]];
			return map ? map(v) : v;
		});
	},

	redraw: function(full) {
		var self = this;
		var chans = this.channels();
		hfc.applyTheme(this.root, hfc.theme());
		this.fillSections();
		this.updateLive();
		if (!chans.length) {
			this.emptyEl.style.display = '';
			this.gridEl.style.display = 'none';
			this.tilesEl.style.display = 'none';
			while (this.emptyEl.firstChild)
				this.emptyEl.removeChild(this.emptyEl.firstChild);
			this.emptyEl.appendChild(E('div', {}, [
				E('strong', {}, _('Данных пока нет.')), E('br'),
				_('Графики пишет нативный движок раз в 2 секунды для каналов секций vpn и proxy. Проверьте, что движок запущен (Обзор), и подождите несколько секунд.')
			]));
			return;
		}
		this.emptyEl.style.display = 'none';
		this.gridEl.style.display = '';
		this.tilesEl.style.display = '';

		var win = this.window();
		var mk = function(field, map, nullLabel) {
			return chans.map(function(c) {
				return { key: c.key, name: c.label, color: c.color, values: self.pick(win, c.key, field, map), nullLabel: nullLabel };
			});
		};
		var base = { t: win.t, from: win.from, to: win.to, step: win.step };
		this.charts.rx.setData(Object.assign({ series: mk('rx', bits) }, base));
		this.charts.tx.setData(Object.assign({ series: mk('tx', bits) }, base));
		this.charts.active.setData(Object.assign({ series: mk('active') }, base));
		this.charts.newc.setData(Object.assign({ series: mk('new', function(v) { return v == null ? null : v / 100; }) }, base));

		var regions = [];
		var delaySeries = chans.map(function(c) {
			var raw = self.pick(win, c.key, 'delay');
			var open = null;
			for (var i = 0; i < raw.length; i++) {
				if (raw[i] === -1 && open == null)
					open = win.t[i];
				if (raw[i] !== -1 && open != null) {
					regions.push({ key: c.key, from: open, to: win.t[i] });
					open = null;
				}
			}
			if (open != null)
				regions.push({ key: c.key, from: open, to: win.t[win.t.length - 1] + win.step });
			return {
				key: c.key, name: c.label, color: c.color, nullLabel: _('нет ответа'),
				values: raw.map(function(v) { return v > 0 ? v : null; })
			};
		});
		this.charts.delay.setData(Object.assign({ series: delaySeries, regions: regions }, base));

		this.renderShare(chans, win);
		this.renderTiles(chans, win);
	},

	fillSections: function() {
		var src = (this.fine && this.fine.channels) || [];
		var names = [];
		src.forEach(function(c) {
			if (names.indexOf(c.section) < 0)
				names.push(c.section);
		});
		var sig = names.join(',');
		if (sig === this._secSig)
			return;
		this._secSig = sig;
		var sel = this.sectionSel;
		while (sel.firstChild)
			sel.removeChild(sel.firstChild);
		sel.appendChild(E('option', { 'value': '' }, _('Все')));
		var self = this;
		names.forEach(function(n) {
			sel.appendChild(E('option', { 'value': n, 'selected': n === self.section ? '' : null }, n));
		});
		sel.parentNode.style.display = names.length > 1 ? '' : 'none';
	},

	renderShare: function(chans, win) {
		var self = this;
		var totals = chans.map(function(c) {
			var rx = self.pick(win, c.key, 'rx'), tx = self.pick(win, c.key, 'tx');
			var sum = 0;
			for (var i = 0; i < rx.length; i++) {
				var dt = i > 0 ? Math.min(win.t[i] - win.t[i - 1], win.step * 2) : win.step;
				if (win.t[i] < win.from)
					continue;
				sum += ((rx[i] || 0) + (tx[i] || 0)) * dt;
			}
			return { c: c, bytes: sum };
		});
		var all = totals.reduce(function(a, b) { return a + b.bytes; }, 0);
		while (this.shareEl.firstChild)
			this.shareEl.removeChild(this.shareEl.firstChild);
		this.shareEl.appendChild(E('p', { 'class': 'hft-share__title' }, [
			_('Распределение трафика за период'),
			E('span', { 'class': 'hfc-panel__desc' }, hfc.UNITS.bytes(all))
		]));
		var bar = E('div', { 'class': 'hft-share__bar' });
		var legend = E('div', { 'class': 'hft-share__legend' });
		totals.forEach(function(x) {
			var pct = all > 0 ? x.bytes / all * 100 : 0;
			if (pct > 0)
				bar.appendChild(E('span', {
					'style': 'width:' + pct + '%;background:' + x.c.color,
					'title': x.c.label + ': ' + hfc.UNITS.percent(pct) + ', ' + hfc.UNITS.bytes(x.bytes)
				}));
			legend.appendChild(E('span', { 'class': 'hft-share__item' }, [
				E('span', { 'class': 'hfc-sw', 'style': 'background:' + x.c.color }),
				x.c.label,
				E('b', {}, hfc.UNITS.percent(pct)),
				E('span', { 'style': 'opacity:.65' }, hfc.UNITS.bytes(x.bytes))
			]));
		});
		this.shareEl.appendChild(bar);
		this.shareEl.appendChild(legend);
	},

	redrawTiles: function() {
		if (this._tilesArgs)
			this.renderTiles(this._tilesArgs[0], this._tilesArgs[1]);
	},

	renderTiles: function(chans, win) {
		var self = this;
		this._tilesArgs = [chans, win];
		var totals = (this.fine && this.fine.totals) || {};
		var keys = chans.map(function(c) { return c.key; });
		var nodes = chans.map(function(c) {
			var tot = totals[c.key] || {};
			var rx = self.pick(win, c.key, 'rx'), tx = self.pick(win, c.key, 'tx');
			var lastRx = null, lastTx = null;
			for (var i = rx.length - 1; i >= 0; i--)
				if (rx[i] != null) {
					lastRx = rx[i];
					lastTx = tx[i];
					break;
				}
			var state = tot.delay > 0 ? 'up' : (tot.delay === -1 ? 'down' : 'na');
			var pill = { up: _('Работает'), down: _('Недоступен'), na: _('Нет проверки') }[state];
			var spark = E('canvas', { 'class': 'hfc-spark' });
			var sum = rx.map(function(v, i) { return v == null && tx[i] == null ? null : (v || 0) + (tx[i] || 0); });
			var tile = E('div', {
				'class': 'hft-tile' + (self.group.hidden[c.key] ? ' hft-tile--off' : ''),
				'title': _('Клик: показать только этот канал. Ctrl/Cmd+клик: скрыть или показать'),
				'click': function(ev) { self.group.toggle(c.key, !(ev.ctrlKey || ev.metaKey), keys); }
			}, [
				E('span', { 'class': 'hft-tile__stripe', 'style': 'background:' + c.color }),
				E('div', { 'class': 'hft-tile__head' }, [
					E('div', {}, [
						E('div', { 'class': 'hft-tile__name' }, c.name),
						E('div', { 'class': 'hft-tile__sub' }, [
							KIND_LABEL[c.kind] || c.kind || '', c.iface ? ' · ' + c.iface : '', ' · ', c.section
						])
					]),
					E('span', { 'class': 'hft-pill hft-pill--' + state }, pill)
				]),
				E('div', { 'class': 'hft-tile__nums' }, [
					numCell(_('Вход'), hfc.UNITS.bps(bits(lastRx))),
					numCell(_('Выход'), hfc.UNITS.bps(bits(lastTx))),
					numCell(_('Задержка'), tot.delay > 0 ? hfc.UNITS.ms(tot.delay) : '—'),
					numCell(_('Соединений'), String(tot.active != null ? tot.active : '—'))
				]),
				spark,
				E('div', { 'class': 'hft-tile__foot' }, [
					E('span', {}, '↓ ' + hfc.UNITS.bytes(tot.rx || 0) + '  ↑ ' + hfc.UNITS.bytes(tot.tx || 0) + '  · ' +
						_('всего соединений') + ' ' + (tot.conns || 0))
				].concat((c.lists || []).map(function(l) { return E('span', { 'class': 'hft-chip' }, l.replace(/^user:/, '')); })))
			]);
			tile._spark = [spark, sum, c.color];
			return tile;
		});
		while (this.tilesEl.firstChild)
			this.tilesEl.removeChild(this.tilesEl.firstChild);
		nodes.forEach(function(n) { self.tilesEl.appendChild(n); });
		nodes.forEach(function(n) { hfc.drawSpark(n._spark[0], n._spark[1], n._spark[2]); });
	}
});
