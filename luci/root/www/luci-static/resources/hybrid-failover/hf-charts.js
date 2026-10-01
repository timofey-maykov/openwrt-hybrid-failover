'use strict';
'require baseclass';

/*
 * Time series panels in the spirit of Grafana: canvas lines with gradient
 * fill, a legend table with last/avg/max, a crosshair and tooltip shared by
 * every panel of a group, drag to zoom, click a legend row to isolate a
 * series (Ctrl/Cmd+click toggles it). No external libraries: the router
 * serves everything.
 */

var PALETTE = [
	'#73BF69', '#F2CC0C', '#8AB8FF', '#FF780A', '#F2495C', '#5794F2',
	'#B877D9', '#FF9830', '#96D98D', '#FADE2A', '#CA95E5', '#FFB357',
	'#56A64B', '#E0B400', '#3274D9', '#FA6400', '#C4162A', '#8F3BB8'
];

var CSS = [
	'.hfc-panel { position: relative; border-radius: 6px; border: 1px solid var(--hfc-border); background: var(--hfc-bg); color: var(--hfc-text); padding: 10px 12px 8px; box-sizing: border-box; min-width: 0; }',
	'.hfc-panel__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin: 0 0 6px; min-height: 22px; }',
	'.hfc-panel__title { font-size: 13px; font-weight: 600; letter-spacing: .01em; margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }',
	'.hfc-panel__desc { font-size: 11px; color: var(--hfc-muted); margin-left: 6px; font-weight: 400; }',
	'.hfc-panel__tools { display: flex; gap: 4px; }',
	'.hfc-tool { appearance: none; border: 1px solid var(--hfc-border); background: transparent; color: var(--hfc-muted); border-radius: 4px; font-size: 11px; padding: 2px 7px; cursor: pointer; line-height: 16px; }',
	'.hfc-tool:hover, .hfc-tool--on { color: var(--hfc-text); border-color: var(--hfc-accent); }',
	'.hfc-canvas-wrap { position: relative; width: 100%; cursor: crosshair; user-select: none; -webkit-user-select: none; touch-action: pan-y; }',
	'.hfc-canvas-wrap canvas { display: block; width: 100%; }',
	'.hfc-tip { position: absolute; z-index: 30; pointer-events: none; min-width: 170px; max-width: 320px; background: var(--hfc-tip-bg); color: var(--hfc-text); border: 1px solid var(--hfc-border); border-radius: 4px; box-shadow: 0 4px 18px rgba(0,0,0,.35); padding: 6px 8px; font-size: 12px; line-height: 1.5; display: none; }',
	'.hfc-tip__time { font-size: 11px; color: var(--hfc-muted); margin-bottom: 3px; }',
	'.hfc-tip__row { display: flex; align-items: center; gap: 6px; white-space: nowrap; }',
	'.hfc-tip__row--hl { font-weight: 700; }',
	'.hfc-tip__name { flex: 1; overflow: hidden; text-overflow: ellipsis; }',
	'.hfc-tip__val { font-variant-numeric: tabular-nums; margin-left: 10px; }',
	'.hfc-sw { display: inline-block; width: 12px; height: 3px; border-radius: 2px; flex: none; }',
	'.hfc-legend { margin-top: 6px; font-size: 12px; max-height: 168px; overflow-y: auto; }',
	'.hfc-legend table { width: 100%; border-collapse: collapse; }',
	'.hfc-legend th { font-weight: 500; color: var(--hfc-accent-text); text-align: right; padding: 2px 6px; font-size: 11px; white-space: nowrap; }',
	'.hfc-legend th:first-child { text-align: left; }',
	'.hfc-legend td { padding: 2px 6px; text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }',
	'.hfc-legend td:first-child { text-align: left; white-space: normal; }',
	'.hfc-legend tr { cursor: pointer; }',
	'.hfc-legend tbody tr:hover td { background: var(--hfc-hover); }',
	'.hfc-legend tr.hfc-off td { opacity: .38; }',
	'.hfc-legend__name { display: inline-flex; align-items: center; gap: 6px; }',
	'.hfc-empty { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; font-size: 12px; color: var(--hfc-muted); pointer-events: none; }',
	'.hfc-spark { display: block; width: 100%; height: 34px; }'
].join('\n');

var cssInjected = false;

function injectCSS() {
	if (cssInjected)
		return;
	cssInjected = true;
	var st = document.createElement('style');
	st.type = 'text/css';
	st.appendChild(document.createTextNode(CSS));
	document.head.appendChild(st);
}

/* Theme: follow the LuCI theme, dark or light, read from the page background. */
function isDark() {
	var el = document.querySelector('#maincontent') || document.body;
	var bg = '';
	while (el && el !== document.documentElement) {
		bg = getComputedStyle(el).backgroundColor;
		if (bg && !/rgba\(\s*0,\s*0,\s*0,\s*0\s*\)|transparent/.test(bg))
			break;
		el = el.parentElement;
	}
	if (!bg || /transparent|rgba\(\s*0,\s*0,\s*0,\s*0\s*\)/.test(bg))
		bg = getComputedStyle(document.body).backgroundColor;
	var m = bg.match(/\d+(\.\d+)?/g);
	if (!m || m.length < 3)
		return false;
	var l = 0.2126 * m[0] + 0.7152 * m[1] + 0.0722 * m[2];
	return l < 110;
}

function theme() {
	return isDark() ? {
		bg: '#181b1f', tipBg: '#22252b', text: '#ccccdc', muted: '#8e8e9e', grid: 'rgba(204,204,220,.08)',
		axis: 'rgba(204,204,220,.55)', border: 'rgba(204,204,220,.12)', accent: '#3d71d9', accentText: '#6e9fff',
		hover: 'rgba(204,204,220,.06)', cross: 'rgba(204,204,220,.45)', select: 'rgba(120,150,255,.18)'
	} : {
		bg: '#ffffff', tipBg: '#ffffff', text: '#24292e', muted: '#6a737d', grid: 'rgba(36,41,46,.07)',
		axis: 'rgba(36,41,46,.6)', border: 'rgba(36,41,46,.14)', accent: '#3d71d9', accentText: '#1f62e0',
		hover: 'rgba(36,41,46,.04)', cross: 'rgba(36,41,46,.4)', select: 'rgba(61,113,217,.14)'
	};
}

function applyTheme(el, th) {
	el.style.setProperty('--hfc-bg', th.bg);
	el.style.setProperty('--hfc-tip-bg', th.tipBg);
	el.style.setProperty('--hfc-text', th.text);
	el.style.setProperty('--hfc-muted', th.muted);
	el.style.setProperty('--hfc-border', th.border);
	el.style.setProperty('--hfc-accent', th.accent);
	el.style.setProperty('--hfc-accent-text', th.accentText);
	el.style.setProperty('--hfc-hover', th.hover);
}

/* ---- units ---- */

function trimNum(v, digits) {
	var s = v.toFixed(digits);
	if (s.indexOf('.') >= 0)
		s = s.replace(/\.?0+$/, '');
	return s;
}

function fmtSI(v, units, base) {
	if (v == null || isNaN(v))
		return '—';
	var abs = Math.abs(v), i = 0;
	while (abs >= base && i < units.length - 1) {
		abs /= base;
		v /= base;
		i++;
	}
	var d = abs >= 100 ? 0 : (abs >= 10 ? 1 : 2);
	return trimNum(v, d) + ' ' + units[i];
}

var UNITS = {
	bps: function(v) { return fmtSI(v, [_('бит/с'), _('кбит/с'), _('Мбит/с'), _('Гбит/с')], 1000); },
	bytes: function(v) { return fmtSI(v, [_('Б'), _('КБ'), _('МБ'), _('ГБ'), _('ТБ')], 1024); },
	ms: function(v) {
		if (v == null || isNaN(v))
			return '—';
		return v >= 1000 ? trimNum(v / 1000, 2) + ' ' + _('с') : Math.round(v) + ' ' + _('мс');
	},
	count: function(v) {
		if (v == null || isNaN(v))
			return '—';
		return v >= 1000 ? fmtSI(v, ['', 'k', 'M'], 1000).trim() : trimNum(v, v < 10 ? 2 : 0);
	},
	rate: function(v) {
		if (v == null || isNaN(v))
			return '—';
		return trimNum(v, v < 10 ? 2 : 1) + ' /' + _('с');
	},
	percent: function(v) { return v == null ? '—' : trimNum(v, 1) + '%'; }
};

function niceStep(raw) {
	var p = Math.pow(10, Math.floor(Math.log10(raw)));
	var f = raw / p;
	return (f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10) * p;
}

function niceTicks(max, count) {
	if (!(max > 0))
		max = 1;
	var step = niceStep(max / Math.max(1, count));
	var top = Math.ceil(max / step) * step;
	var out = [];
	for (var v = 0; v <= top + step / 2; v += step)
		out.push(v);
	return { ticks: out, top: top };
}

var TIME_STEPS = [5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200];

function pad2(n) { return n < 10 ? '0' + n : '' + n; }

function fmtClock(t, withSec) {
	var d = new Date(t * 1000);
	return pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + (withSec ? ':' + pad2(d.getSeconds()) : '');
}

function fmtFull(t) {
	var d = new Date(t * 1000);
	return d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate()) + ' ' + fmtClock(t, true);
}

/* ---- sync groups: shared crosshair and series visibility ---- */

function Group() {
	this.charts = [];
	this.hidden = {};
	this.hoverT = null;
}

Group.prototype.add = function(c) { this.charts.push(c); };
Group.prototype.remove = function(c) {
	var i = this.charts.indexOf(c);
	if (i >= 0)
		this.charts.splice(i, 1);
};
Group.prototype.hover = function(t, src) {
	this.hoverT = t;
	for (var i = 0; i < this.charts.length; i++)
		this.charts[i].setHover(t, this.charts[i] === src);
};
Group.prototype.toggle = function(key, isolate, keys) {
	if (isolate) {
		var onlyThis = !this.hidden[key];
		for (var i = 0; i < keys.length; i++)
			if (keys[i] !== key && !this.hidden[keys[i]])
				onlyThis = false;
		this.hidden = {};
		if (!onlyThis)
			for (var j = 0; j < keys.length; j++)
				if (keys[j] !== key)
					this.hidden[keys[j]] = true;
	}
	else {
		this.hidden[key] = !this.hidden[key];
	}
	this.redrawAll();
};
Group.prototype.redrawAll = function() {
	for (var i = 0; i < this.charts.length; i++)
		this.charts[i].render();
	if (this.onChange)
		this.onChange();
};

/* ---- the time series panel ---- */

function TimeSeries(opts) {
	injectCSS();
	this.opts = Object.assign({
		title: '', desc: '', unit: 'count', height: 200, fill: true, stacked: false,
		stackToggle: false, min: 0, thresholds: [], legend: true, group: null,
		onZoom: null, onReset: null, legendCols: ['last', 'avg', 'max']
	}, opts || {});
	this.group = this.opts.group || new Group();
	this.group.add(this);
	this.data = { t: [], series: [], from: 0, to: 0, step: 2 };
	this.hoverT = null;
	this.drag = null;
	this.build();
}

TimeSeries.prototype.build = function() {
	var self = this;
	var th = theme();
	this.th = th;
	this.el = E('div', { 'class': 'hfc-panel' });
	applyTheme(this.el, th);

	var tools = E('div', { 'class': 'hfc-panel__tools' });
	if (this.opts.stackToggle) {
		this.stackBtn = E('button', {
			'class': 'hfc-tool' + (this.opts.stacked ? ' hfc-tool--on' : ''),
			'title': _('Складывать каналы друг на друга'),
			'click': function() {
				self.opts.stacked = !self.opts.stacked;
				self.stackBtn.classList.toggle('hfc-tool--on', self.opts.stacked);
				self.render();
			}
		}, _('Стек'));
		tools.appendChild(this.stackBtn);
	}
	this.el.appendChild(E('div', { 'class': 'hfc-panel__head' }, [
		E('p', { 'class': 'hfc-panel__title' }, [
			this.opts.title,
			this.opts.desc ? E('span', { 'class': 'hfc-panel__desc' }, this.opts.desc) : ''
		]),
		tools
	]));

	this.wrap = E('div', { 'class': 'hfc-canvas-wrap', 'style': 'height:' + this.opts.height + 'px' });
	this.canvas = document.createElement('canvas');
	this.canvas.style.height = this.opts.height + 'px';
	this.wrap.appendChild(this.canvas);
	this.tip = E('div', { 'class': 'hfc-tip' });
	this.wrap.appendChild(this.tip);
	this.emptyEl = E('div', { 'class': 'hfc-empty' }, _('Нет данных'));
	this.wrap.appendChild(this.emptyEl);
	this.el.appendChild(this.wrap);

	if (this.opts.legend) {
		this.legendEl = E('div', { 'class': 'hfc-legend' });
		this.el.appendChild(this.legendEl);
	}

	this.wrap.addEventListener('mousemove', function(ev) { self.onMove(ev); });
	this.wrap.addEventListener('mouseleave', function() {
		if (!self.drag)
			self.group.hover(null, self);
	});
	this.wrap.addEventListener('mousedown', function(ev) {
		if (ev.button !== 0)
			return;
		var x = self.localX(ev);
		if (x < self.plot.x || x > self.plot.x + self.plot.w)
			return;
		self.drag = { x0: x, x1: x };
		ev.preventDefault();
	});
	this._onUp = function(ev) {
		if (!self.drag)
			return;
		var d = self.drag;
		self.drag = null;
		if (Math.abs(d.x1 - d.x0) > 6 && self.opts.onZoom) {
			var a = self.xToT(Math.min(d.x0, d.x1)), b = self.xToT(Math.max(d.x0, d.x1));
			self.opts.onZoom(Math.floor(a), Math.ceil(b));
		}
		self.render();
	};
	window.addEventListener('mouseup', this._onUp);
	this.wrap.addEventListener('dblclick', function() {
		if (self.opts.onReset)
			self.opts.onReset();
	});
	this.wrap.addEventListener('touchstart', function(ev) { self.onMove(ev.touches[0]); }, { passive: true });
	this.wrap.addEventListener('touchmove', function(ev) { self.onMove(ev.touches[0]); }, { passive: true });

	if (window.ResizeObserver) {
		this.ro = new ResizeObserver(function() { self.render(); });
		this.ro.observe(this.wrap);
	}
	else {
		window.addEventListener('resize', function() { self.render(); });
	}
};

TimeSeries.prototype.destroy = function() {
	this.group.remove(this);
	if (this.ro)
		this.ro.disconnect();
	window.removeEventListener('mouseup', this._onUp);
};

/*
 * data: { t: [unix sec], step: seconds, from, to,
 *         series: [{ key, name, color, values: [number|null] }] }
 */
TimeSeries.prototype.setData = function(data) {
	this.data = data;
	this.render();
};

TimeSeries.prototype.localX = function(ev) {
	var r = this.canvas.getBoundingClientRect();
	return ev.clientX - r.left;
};

TimeSeries.prototype.xToT = function(x) {
	var p = this.plot;
	return this.data.from + (x - p.x) / p.w * (this.data.to - this.data.from);
};

TimeSeries.prototype.tToX = function(t) {
	var p = this.plot;
	return p.x + (t - this.data.from) / Math.max(1, this.data.to - this.data.from) * p.w;
};

TimeSeries.prototype.visibleSeries = function() {
	var hidden = this.group.hidden, out = [];
	for (var i = 0; i < this.data.series.length; i++)
		if (!hidden[this.data.series[i].key])
			out.push(this.data.series[i]);
	return out;
};

/* Nearest sample index to time t, or -1. */
TimeSeries.prototype.indexAt = function(t) {
	var ts = this.data.t;
	if (!ts.length || t == null)
		return -1;
	var lo = 0, hi = ts.length - 1;
	while (lo < hi) {
		var mid = (lo + hi) >> 1;
		if (ts[mid] < t)
			lo = mid + 1;
		else
			hi = mid;
	}
	if (lo > 0 && Math.abs(ts[lo - 1] - t) < Math.abs(ts[lo] - t))
		lo--;
	if (Math.abs(ts[lo] - t) > (this.data.step || 2) * 3)
		return -1;
	return lo;
};

TimeSeries.prototype.onMove = function(ev) {
	if (!this.plot)
		return;
	var x = this.localX(ev);
	if (this.drag) {
		this.drag.x1 = Math.max(this.plot.x, Math.min(this.plot.x + this.plot.w, x));
		this.render();
		return;
	}
	if (x < this.plot.x || x > this.plot.x + this.plot.w) {
		this.group.hover(null, this);
		return;
	}
	this._hoverY = ev.clientY - this.canvas.getBoundingClientRect().top;
	this.group.hover(this.xToT(x), this);
};

TimeSeries.prototype.setHover = function(t, isSource) {
	this.hoverT = t;
	this.hoverSource = isSource;
	this.render();
};

/* Values as drawn: stacked sums when stacking. */
TimeSeries.prototype.drawnValues = function(vis) {
	var n = this.data.t.length, out = [], acc = new Array(n);
	for (var k = 0; k < n; k++)
		acc[k] = 0;
	for (var i = 0; i < vis.length; i++) {
		var v = vis[i].values, arr = new Array(n);
		for (var j = 0; j < n; j++) {
			var x = v[j];
			if (this.opts.stacked) {
				acc[j] += (x == null ? 0 : x);
				arr[j] = x == null && !acc[j] ? null : acc[j];
			}
			else {
				arr[j] = x;
			}
		}
		out.push(arr);
	}
	return out;
};

TimeSeries.prototype.render = function() {
	var self = this;
	var th = theme();
	if (th.bg !== this.th.bg) {
		this.th = th;
		applyTheme(this.el, th);
	}
	var cssW = this.wrap.clientWidth, cssH = this.opts.height;
	if (!cssW)
		return;
	var dpr = window.devicePixelRatio || 1;
	if (this.canvas.width !== Math.round(cssW * dpr) || this.canvas.height !== Math.round(cssH * dpr)) {
		this.canvas.width = Math.round(cssW * dpr);
		this.canvas.height = Math.round(cssH * dpr);
	}
	var ctx = this.canvas.getContext('2d');
	ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
	ctx.clearRect(0, 0, cssW, cssH);

	var fmt = UNITS[this.opts.unit] || UNITS.count;
	var vis = this.visibleSeries();
	var drawn = this.drawnValues(vis);
	var ts = this.data.t;

	var max = 0;
	for (var i = 0; i < drawn.length; i++)
		for (var j = 0; j < drawn[i].length; j++)
			if (drawn[i][j] != null && ts[j] >= this.data.from && drawn[i][j] > max)
				max = drawn[i][j];
	for (var k = 0; k < this.opts.thresholds.length; k++)
		if (this.opts.thresholds[k].value <= max * 1.6)
			max = Math.max(max, this.opts.thresholds[k].value * 1.08);

	var yt = niceTicks(max * 1.05, Math.max(2, Math.floor(cssH / 45)));
	ctx.font = '11px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
	var labelW = 0;
	for (var a = 0; a < yt.ticks.length; a++)
		labelW = Math.max(labelW, ctx.measureText(fmt(yt.ticks[a])).width);
	var p = { x: Math.ceil(labelW) + 10, y: 6, w: 0, h: 0 };
	p.w = cssW - p.x - 8;
	p.h = cssH - p.y - 20;
	this.plot = p;
	var yOf = function(v) { return p.y + p.h - (v / yt.top) * p.h; };

	// grid and y labels
	ctx.lineWidth = 1;
	ctx.textAlign = 'right';
	ctx.textBaseline = 'middle';
	for (var b = 0; b < yt.ticks.length; b++) {
		var y = Math.round(yOf(yt.ticks[b])) + 0.5;
		ctx.strokeStyle = th.grid;
		ctx.beginPath();
		ctx.moveTo(p.x, y);
		ctx.lineTo(p.x + p.w, y);
		ctx.stroke();
		ctx.fillStyle = th.axis;
		ctx.fillText(fmt(yt.ticks[b]), p.x - 6, y);
	}

	// x ticks
	var span = Math.max(1, this.data.to - this.data.from);
	var want = Math.max(2, Math.floor(p.w / 90));
	var tstep = TIME_STEPS[TIME_STEPS.length - 1];
	for (var c = 0; c < TIME_STEPS.length; c++)
		if (span / TIME_STEPS[c] <= want) {
			tstep = TIME_STEPS[c];
			break;
		}
	var tzOff = new Date().getTimezoneOffset() * 60;
	var first = Math.ceil((this.data.from - tzOff) / tstep) * tstep + tzOff;
	ctx.textAlign = 'center';
	ctx.textBaseline = 'top';
	for (var tt = first; tt <= this.data.to; tt += tstep) {
		var x = Math.round(this.tToX(tt)) + 0.5;
		ctx.strokeStyle = th.grid;
		ctx.beginPath();
		ctx.moveTo(x, p.y);
		ctx.lineTo(x, p.y + p.h);
		ctx.stroke();
		ctx.fillStyle = th.axis;
		ctx.fillText(fmtClock(tt, tstep < 60), x, p.y + p.h + 5);
	}

	// thresholds: dashed line and a faint band above it
	for (var d = 0; d < this.opts.thresholds.length; d++) {
		var thr = this.opts.thresholds[d];
		if (thr.value > yt.top)
			continue;
		var ty = Math.round(yOf(thr.value)) + 0.5;
		ctx.fillStyle = thr.color.replace(')', ',.06)').replace('rgb(', 'rgba(');
		ctx.fillRect(p.x, p.y, p.w, ty - p.y);
		ctx.setLineDash([4, 4]);
		ctx.strokeStyle = thr.color;
		ctx.beginPath();
		ctx.moveTo(p.x, ty);
		ctx.lineTo(p.x + p.w, ty);
		ctx.stroke();
		ctx.setLineDash([]);
	}

	// annotations: shaded regions (e.g. a channel down)
	var regions = this.data.regions || [];
	for (var r = 0; r < regions.length; r++) {
		var rg = regions[r];
		if (this.group.hidden[rg.key])
			continue;
		var rx0 = Math.max(p.x, this.tToX(rg.from)), rx1 = Math.min(p.x + p.w, this.tToX(rg.to));
		if (rx1 <= rx0)
			continue;
		ctx.fillStyle = 'rgba(242,73,92,.10)';
		ctx.fillRect(rx0, p.y, Math.max(2, rx1 - rx0), p.h);
		ctx.fillStyle = 'rgba(242,73,92,.85)';
		ctx.fillRect(rx0, p.y + p.h - 3, Math.max(2, rx1 - rx0), 3);
	}

	ctx.save();
	ctx.beginPath();
	ctx.rect(p.x, p.y, p.w, p.h);
	ctx.clip();
	var gap = (this.data.step || 2) * 2.5;
	var hasPoints = false;
	// fills first, lines on top
	for (var pass = 0; pass < 2; pass++) {
		for (var s = drawn.length - 1; s >= 0; s--) {
			var vals = drawn[s], color = vis[s].color;
			var below = this.opts.stacked && s > 0 ? drawn[s - 1] : null;
			var segs = [], seg = null;
			for (var q = 0; q < ts.length; q++) {
				if (vals[q] == null || (seg && ts[q] - ts[q - 1] > gap)) {
					if (seg)
						segs.push(seg);
					seg = null;
					if (vals[q] == null)
						continue;
				}
				if (!seg)
					seg = [];
				seg.push(q);
			}
			if (seg)
				segs.push(seg);
			for (var g = 0; g < segs.length; g++) {
				var sg = segs[g];
				hasPoints = true;
				ctx.beginPath();
				for (var m = 0; m < sg.length; m++) {
					var px = this.tToX(ts[sg[m]]), py = yOf(vals[sg[m]]);
					if (m === 0)
						ctx.moveTo(px, py);
					else
						ctx.lineTo(px, py);
				}
				if (pass === 0) {
					if (!this.opts.fill && !this.opts.stacked)
						continue;
					if (below) {
						for (var bm = sg.length - 1; bm >= 0; bm--)
							ctx.lineTo(this.tToX(ts[sg[bm]]), yOf(below[sg[bm]] || 0));
					}
					else {
						ctx.lineTo(this.tToX(ts[sg[sg.length - 1]]), p.y + p.h);
						ctx.lineTo(this.tToX(ts[sg[0]]), p.y + p.h);
					}
					ctx.closePath();
					var grad = ctx.createLinearGradient(0, p.y, 0, p.y + p.h);
					grad.addColorStop(0, hexA(color, this.opts.stacked ? 0.55 : 0.32));
					grad.addColorStop(1, hexA(color, this.opts.stacked ? 0.30 : 0.02));
					ctx.fillStyle = grad;
					ctx.fill();
				}
				else {
					ctx.strokeStyle = color;
					ctx.lineWidth = 1.6;
					ctx.lineJoin = 'round';
					ctx.stroke();
					if (sg.length === 1) {
						ctx.fillStyle = color;
						ctx.beginPath();
						ctx.arc(this.tToX(ts[sg[0]]), yOf(vals[sg[0]]), 2.2, 0, Math.PI * 2);
						ctx.fill();
					}
				}
			}
		}
	}
	ctx.restore();
	this.emptyEl.style.display = hasPoints ? 'none' : '';

	// drag selection
	if (this.drag) {
		ctx.fillStyle = th.select;
		ctx.fillRect(Math.min(this.drag.x0, this.drag.x1), p.y, Math.abs(this.drag.x1 - this.drag.x0), p.h);
	}

	// crosshair, points and tooltip
	var idx = this.indexAt(this.hoverT);
	if (this.hoverT != null && idx >= 0 && !this.drag) {
		var hx = Math.round(this.tToX(ts[idx])) + 0.5;
		ctx.strokeStyle = th.cross;
		ctx.lineWidth = 1;
		ctx.setLineDash([3, 3]);
		ctx.beginPath();
		ctx.moveTo(hx, p.y);
		ctx.lineTo(hx, p.y + p.h);
		ctx.stroke();
		ctx.setLineDash([]);
		var rows = [];
		for (var h = 0; h < vis.length; h++) {
			var dv = drawn[h][idx];
			if (dv != null) {
				ctx.fillStyle = vis[h].color;
				ctx.strokeStyle = th.bg;
				ctx.lineWidth = 2;
				ctx.beginPath();
				ctx.arc(hx, yOf(dv), 3.5, 0, Math.PI * 2);
				ctx.fill();
				ctx.stroke();
			}
			rows.push({ s: vis[h], v: vis[h].values[idx], y: dv == null ? null : yOf(dv) });
		}
		if (this.hoverSource)
			this.showTip(hx, ts[idx], rows, fmt);
		else
			this.tip.style.display = 'none';
	}
	else {
		this.tip.style.display = 'none';
	}

	this.renderLegend(fmt);
};

TimeSeries.prototype.showTip = function(x, t, rows, fmt) {
	var near = null, best = 1e9;
	for (var i = 0; i < rows.length; i++)
		if (rows[i].y != null && Math.abs(rows[i].y - this._hoverY) < best) {
			best = Math.abs(rows[i].y - this._hoverY);
			near = rows[i];
		}
	rows = rows.slice().sort(function(a, b) { return (b.v || -1) - (a.v || -1); });
	var tip = this.tip;
	while (tip.firstChild)
		tip.removeChild(tip.firstChild);
	tip.appendChild(E('div', { 'class': 'hfc-tip__time' }, fmtFull(t)));
	for (var j = 0; j < rows.length; j++) {
		var rw = rows[j];
		var txt = rw.v == null ? (rw.s.nullLabel || '—') : fmt(rw.v);
		tip.appendChild(E('div', { 'class': 'hfc-tip__row' + (rw === near && best < 24 ? ' hfc-tip__row--hl' : '') }, [
			E('span', { 'class': 'hfc-sw', 'style': 'background:' + rw.s.color }),
			E('span', { 'class': 'hfc-tip__name' }, rw.s.name),
			E('span', { 'class': 'hfc-tip__val' }, txt)
		]));
	}
	tip.style.display = 'block';
	var w = tip.offsetWidth, cw = this.wrap.clientWidth;
	var left = x + 14;
	if (left + w > cw)
		left = x - w - 14;
	tip.style.left = Math.max(0, left) + 'px';
	tip.style.top = Math.max(0, Math.min((this._hoverY || 0) - 20, this.opts.height - tip.offsetHeight)) + 'px';
};

function seriesStats(values, ts, from) {
	var last = null, sum = 0, n = 0, max = null, min = null;
	for (var i = 0; i < values.length; i++) {
		if (ts[i] < from)
			continue;
		var v = values[i];
		if (v == null)
			continue;
		last = v;
		sum += v;
		n++;
		if (max == null || v > max)
			max = v;
		if (min == null || v < min)
			min = v;
	}
	return { last: last, avg: n ? sum / n : null, max: max, min: min, total: sum };
}

TimeSeries.prototype.renderLegend = function(fmt) {
	if (!this.legendEl)
		return;
	var self = this;
	var cols = this.opts.legendCols;
	var labels = { last: _('Последнее'), avg: _('Среднее'), max: _('Макс.'), min: _('Мин.') };
	var keyAll = this.data.series.map(function(s) { return s.key; });
	var sig = JSON.stringify([keyAll, this.group.hidden, this.data.t.length, this.data.t[this.data.t.length - 1], this.data.from]);
	if (sig === this._legendSig)
		return;
	this._legendSig = sig;
	var body = [];
	for (var i = 0; i < this.data.series.length; i++) {
		(function(s) {
			var st = seriesStats(s.values, self.data.t, self.data.from);
			var cells = [E('td', {}, E('span', { 'class': 'hfc-legend__name' }, [
				E('span', { 'class': 'hfc-sw', 'style': 'background:' + s.color }), s.name
			]))];
			for (var c = 0; c < cols.length; c++)
				cells.push(E('td', {}, fmt(st[cols[c]])));
			body.push(E('tr', {
				'class': self.group.hidden[s.key] ? 'hfc-off' : '',
				'title': _('Клик: показать только этот канал. Ctrl/Cmd+клик: скрыть или показать'),
				'click': function(ev) { self.group.toggle(s.key, !(ev.ctrlKey || ev.metaKey), keyAll); }
			}, cells));
		})(this.data.series[i]);
	}
	var head = [E('th', {}, '')];
	for (var h = 0; h < cols.length; h++)
		head.push(E('th', {}, labels[cols[h]]));
	while (this.legendEl.firstChild)
		this.legendEl.removeChild(this.legendEl.firstChild);
	this.legendEl.appendChild(E('table', {}, [E('thead', {}, E('tr', {}, head)), E('tbody', {}, body)]));
};

function hexA(hex, alpha) {
	var h = hex.replace('#', '');
	if (h.length === 3)
		h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2];
	var n = parseInt(h, 16);
	return 'rgba(' + ((n >> 16) & 255) + ',' + ((n >> 8) & 255) + ',' + (n & 255) + ',' + alpha + ')';
}

/* Small sparkline for stat tiles. */
function drawSpark(canvas, values, color) {
	var w = canvas.clientWidth, h = canvas.clientHeight;
	if (!w || !h)
		return;
	var dpr = window.devicePixelRatio || 1;
	canvas.width = Math.round(w * dpr);
	canvas.height = Math.round(h * dpr);
	var ctx = canvas.getContext('2d');
	ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
	ctx.clearRect(0, 0, w, h);
	var max = 0;
	for (var i = 0; i < values.length; i++)
		if (values[i] != null && values[i] > max)
			max = values[i];
	if (!values.length)
		return;
	max = max || 1;
	var step = w / Math.max(1, values.length - 1);
	ctx.beginPath();
	for (var j = 0; j < values.length; j++) {
		var x = j * step, y = h - 2 - ((values[j] || 0) / max) * (h - 4);
		if (j === 0)
			ctx.moveTo(x, y);
		else
			ctx.lineTo(x, y);
	}
	ctx.strokeStyle = color;
	ctx.lineWidth = 1.5;
	ctx.stroke();
	ctx.lineTo(w, h);
	ctx.lineTo(0, h);
	ctx.closePath();
	var grad = ctx.createLinearGradient(0, 0, 0, h);
	grad.addColorStop(0, hexA(color, 0.35));
	grad.addColorStop(1, hexA(color, 0));
	ctx.fillStyle = grad;
	ctx.fill();
}

return baseclass.extend({
	PALETTE: PALETTE,
	UNITS: UNITS,
	Group: Group,
	TimeSeries: TimeSeries,
	drawSpark: drawSpark,
	seriesStats: seriesStats,
	theme: theme,
	applyTheme: applyTheme,
	injectCSS: injectCSS,
	hexA: hexA,
	fmtFull: fmtFull
});
