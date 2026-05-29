'use strict';
'require view';
'require poll';
'require rpc';
'require ui';

var callStatus = rpc.declare({
	object: 'hybrid-failover',
	method: 'status'
});

var callHealth = rpc.declare({
	object: 'hybrid-failover',
	method: 'health'
});

var callHistory = rpc.declare({
	object: 'hybrid-failover',
	method: 'history'
});

var callCheckFakeip = rpc.declare({
	object: 'hybrid-failover',
	method: 'check_fakeip'
});

var callExportHistory = rpc.declare({
	object: 'hybrid-failover',
	method: 'export_history'
});

var callSwitchProxy = rpc.declare({
	object: 'hybrid-failover',
	method: 'switch_proxy',
	params: [ 'section', 'outbound' ]
});

var HF_DELAY_MAX = 40;

var HF_MON_CSS = [
	'.hf-mon { max-width: 1100px; margin: 0 auto 24px; color: var(--cbi-section-text-color, inherit); }',
	'.hf-mon-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 16px; }',
	'.hf-mon-toolbar .hf-mon-updated { opacity: 0.75; font-size: 12px; margin-left: auto; }',
	'.hf-mon-banner { padding: 10px 14px; border-radius: 8px; margin-bottom: 16px; font-size: 13px; }',
	'.hf-mon-banner--ok { background: rgba(60,186,84,.12); border: 1px solid rgba(60,186,84,.4); }',
	'.hf-mon-banner--warn { background: rgba(240,173,78,.12); border: 1px solid rgba(240,173,78,.45); }',
	'.hf-mon-banner--bad { background: rgba(231,76,60,.12); border: 1px solid rgba(231,76,60,.45); }',
	'.hf-mon-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; margin-bottom: 18px; }',
	'.hf-mon-card { border: 1px solid var(--border-color, rgba(127,127,127,.35)); border-radius: 8px; padding: 12px 14px; background: var(--cbi-section-background-color, rgba(127,127,127,.06)); }',
	'.hf-mon-card__label { font-size: 11px; text-transform: uppercase; letter-spacing: .04em; opacity: .7; margin-bottom: 6px; }',
	'.hf-mon-card__value { font-size: 15px; font-weight: 600; }',
	'.hf-mon-card--ok { border-left: 4px solid #3cba54; }',
	'.hf-mon-card--warn { border-left: 4px solid #f0ad4e; }',
	'.hf-mon-card--bad { border-left: 4px solid #e74c3c; }',
	'.hf-mon-card--neutral { border-left: 4px solid var(--border-color, rgba(127,127,127,.5)); }',
	'.hf-mon-panels { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-bottom: 18px; }',
	'@media (max-width: 720px) { .hf-mon-panels { grid-template-columns: 1fr; } }',
	'.hf-mon-panel { border: 1px solid var(--border-color, rgba(127,127,127,.35)); border-radius: 8px; padding: 14px 16px; }',
	'.hf-mon-panel h4 { margin: 0 0 12px; font-size: 14px; font-weight: 600; }',
	'.hf-mon-kv { display: grid; grid-template-columns: auto 1fr; gap: 6px 14px; font-size: 13px; }',
	'.hf-mon-kv dt { opacity: .75; margin: 0; }',
	'.hf-mon-kv dd { margin: 0; font-weight: 500; word-break: break-word; }',
	'.hf-mon-section { margin-bottom: 20px; }',
	'.hf-mon-section h3 { margin: 0 0 10px; font-size: 15px; font-weight: 600; }',
	'.hf-mon-table { width: 100%; border-collapse: collapse; font-size: 13px; }',
	'.hf-mon-table th, .hf-mon-table td { padding: 8px 10px; text-align: left; border-bottom: 1px solid var(--border-color, rgba(127,127,127,.25)); }',
	'.hf-mon-table th { font-size: 11px; text-transform: uppercase; letter-spacing: .03em; opacity: .75; font-weight: 600; }',
	'.hf-mon-table tr.hf-mon-row--active { background: rgba(60,186,84,.08); }',
	'.hf-mon-badge { display: inline-block; padding: 2px 8px; border-radius: 999px; font-size: 11px; font-weight: 600; }',
	'.hf-mon-badge--ok { background: rgba(60,186,84,.2); color: #2d8a3e; }',
	'.hf-mon-badge--bad { background: rgba(231,76,60,.2); color: #c0392b; }',
	'.hf-mon-badge--warn { background: rgba(240,173,78,.25); color: #b8860b; }',
	'.hf-mon-badge--info { background: rgba(52,152,219,.2); color: #2980b9; }',
	'.hf-mon-latency { display: flex; align-items: center; gap: 8px; min-width: 120px; }',
	'.hf-mon-latency-bar { flex: 1; height: 6px; border-radius: 3px; background: var(--border-color, rgba(127,127,127,.25)); overflow: hidden; max-width: 100px; }',
	'.hf-mon-latency-bar > span { display: block; height: 100%; border-radius: 3px; background: #3cba54; }',
	'.hf-mon-latency-bar > span.warn { background: #f0ad4e; }',
	'.hf-mon-latency-bar > span.bad { background: #e74c3c; }',
	'.hf-mon-empty { opacity: .7; font-size: 13px; padding: 12px 0; }',
	'.hf-mon-tag { font-family: ui-monospace, monospace; font-size: 12px; }',
	'.hf-mon-spark { vertical-align: middle; }',
	'.hf-mon-switch { display: flex; flex-wrap: wrap; gap: 10px; align-items: flex-end; margin-bottom: 16px; }',
	'.hf-mon-switch label { font-size: 12px; display: block; margin-bottom: 4px; }',
	'.hf-mon-switch select { min-width: 200px; }'
].join('\n');

function emptyNode(node) {
	while (node && node.firstChild)
		node.removeChild(node.firstChild);
}

function unwrapData(res) {
	if (!res)
		return null;
	if (res.data != null)
		return res.data;
	return res;
}

function overallState(data) {
	if (!data || typeof data !== 'object')
		return 'unknown';
	var critical = data.singbox_running && data.nft_ok && data.clash_ok;
	if (!critical)
		return 'down';
	if (data.fakeip_ok === false)
		return 'degraded';
	if (data.errors && data.errors.length)
		return 'degraded';
	return 'up';
}

function badge(ok, okLabel, badLabel) {
	var cls = ok ? 'hf-mon-badge hf-mon-badge--ok' : 'hf-mon-badge hf-mon-badge--bad';
	return E('span', { 'class': cls }, ok ? okLabel : badLabel);
}

function card(label, value, state) {
	state = state || 'neutral';
	return E('div', { 'class': 'hf-mon-card hf-mon-card--' + state }, [
		E('div', { 'class': 'hf-mon-card__label' }, label),
		E('div', { 'class': 'hf-mon-card__value' }, value)
	]);
}

function latencyBar(ms, maxMs) {
	maxMs = maxMs || 500;
	var pct = 0;
	var cls = '';
	if (ms > 0) {
		pct = Math.min(100, Math.round((ms / maxMs) * 100));
		if (ms > 400)
			cls = 'bad';
		else if (ms > 200)
			cls = 'warn';
	}
	return E('div', { 'class': 'hf-mon-latency' }, [
		E('div', { 'class': 'hf-mon-latency-bar' }, [
			E('span', { 'class': cls, 'style': 'width:' + pct + '%' })
		]),
		E('span', {}, ms > 0 ? (ms + ' ms') : '-')
	]);
}

function recordDelayHistory(channels) {
	if (!channels || !window.localStorage)
		return;
	for (var i = 0; i < channels.length; i++) {
		var ch = channels[i];
		if (!ch.name || !ch.delay_ms)
			continue;
		var key = 'hf_delay_' + ch.name;
		var list = [];
		try {
			list = JSON.parse(localStorage.getItem(key) || '[]');
		} catch (e) { list = []; }
		list.push({ t: Date.now(), d: ch.delay_ms });
		if (list.length > HF_DELAY_MAX)
			list = list.slice(list.length - HF_DELAY_MAX);
		localStorage.setItem(key, JSON.stringify(list));
	}
}

function clearDelayHistory() {
	if (!window.localStorage)
		return;
	var keys = [];
	for (var i = 0; i < localStorage.length; i++) {
		var k = localStorage.key(i);
		if (k && k.indexOf('hf_delay_') === 0)
			keys.push(k);
	}
	for (var j = 0; j < keys.length; j++)
		localStorage.removeItem(keys[j]);
}

function buildSparklineSVG(name) {
	var points = [];
	try {
		points = JSON.parse(localStorage.getItem('hf_delay_' + name) || '[]');
	} catch (e) { points = []; }
	if (!points.length)
		return E('span', { 'class': 'hf-mon-empty' }, '-');
	var w = 80, h = 20, maxD = 1;
	for (var i = 0; i < points.length; i++)
		if (points[i].d > maxD)
			maxD = points[i].d;
	var coords = [];
	for (var j = 0; j < points.length; j++) {
		var x = points.length === 1 ? w / 2 : (j / (points.length - 1)) * w;
		var y = h - (points[j].d / maxD) * (h - 2) - 1;
		coords.push(x.toFixed(1) + ',' + y.toFixed(1));
	}
	return E('svg', {
		'class': 'hf-mon-spark',
		'width': String(w),
		'height': String(h),
		'viewBox': '0 0 ' + w + ' ' + h
	}, [
		E('polyline', {
			'fill': 'none',
			'stroke': '#3cba54',
			'stroke-width': '1.5',
			'points': coords.join(' ')
		})
	]);
}

function exportHistoryFile() {
	return callExportHistory().then(function(res) {
		var data = unwrapData(res);
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

function maxChannelDelay(channels) {
	var max = 300;
	if (!channels)
		return max;
	for (var i = 0; i < channels.length; i++) {
		if (channels[i].delay_ms > max)
			max = channels[i].delay_ms;
	}
	return Math.max(max, 100);
}

function formatEventTime(raw) {
	if (!raw)
		return '-';
	var d = new Date(raw);
	if (isNaN(d.getTime()))
		return String(raw);
	return d.toLocaleString();
}

function policyHint(policy) {
	switch (policy) {
	case 'outage-only':
		return _('VPN пока probe OK; при сбоях: резервы');
	case 'prefer-primary':
		return _('Предпочитать VPN; быстрый возврат');
	case 'fastest':
		return _('Выбор самого быстрого канала (urltest)');
	default:
		return policy || '-';
	}
}

function buildSummaryBanner(data) {
	var state = overallState(data);
	var title, cls, text;
	if (state === 'up') {
		cls = 'hf-mon-banner hf-mon-banner--ok';
		title = _('Маршрутизация активна');
		text = _('Все критичные компоненты работают.');
	} else if (state === 'degraded') {
		cls = 'hf-mon-banner hf-mon-banner--warn';
		title = _('Частичная деградация');
		text = _('Сервис работает, но есть предупреждения: см. карточки ниже.');
	} else if (state === 'down') {
		cls = 'hf-mon-banner hf-mon-banner--bad';
		title = _('Маршрутизация неактивна');
		text = _('sing-box, nft или Clash API недоступны.');
	} else {
		cls = 'hf-mon-banner';
		title = _('Нет данных');
		text = _('Не удалось получить статус с роутера.');
	}
	var children = [E('strong', {}, title + ': '), text];
	if (data && data.active_outbound)
		children.push(E('span', { 'class': 'hf-mon-tag', 'style': 'display:block;margin-top:6px;' },
			_('Активный outbound') + ': ' + data.active_outbound));
	if (data && data.errors && data.errors.length)
		children.push(E('div', { 'style': 'margin-top:8px;color:#c0392b;' }, data.errors.join(' · ')));
	return E('div', { 'class': cls }, children);
}

function buildMetricCards(data) {
	var fakeipState = 'neutral';
	var fakeipVal = _('н/д');
	if (data) {
		if (data.fakeip_skipped)
			fakeipVal = _('пропущено');
		else if (data.fakeip_ok != null) {
			fakeipState = data.fakeip_ok ? 'ok' : 'bad';
			fakeipVal = data.fakeip_ok ? 'OK' : _('ошибка');
		}
	}
	return E('div', { 'class': 'hf-mon-grid' }, [
		card('sing-box', data && data.singbox_running ? _('работает') : _('остановлен'),
			data && data.singbox_running ? 'ok' : 'bad'),
		card('nft / tproxy', data && data.nft_ok ? 'OK' : _('ошибка'),
			data && data.nft_ok ? 'ok' : 'bad'),
		card('Clash API', data && data.clash_ok ? 'OK' : _('недоступен'),
			data && data.clash_ok ? 'ok' : 'bad'),
		card('fakeip DNS', fakeipVal, fakeipState),
		card(_('Активный канал'), (data && data.active_outbound) || '-',
			data && data.active_outbound ? 'ok' : 'neutral')
	]);
}

function kvValue(val) {
	if (val != null && val.nodeType === 1)
		return [val];
	return String(val != null ? val : '-');
}

function buildKvPanel(title, rows) {
	var dl = E('dl', { 'class': 'hf-mon-kv' });
	for (var i = 0; i < rows.length; i++) {
		dl.appendChild(E('dt', {}, rows[i][0]));
		dl.appendChild(E('dd', {}, kvValue(rows[i][1])));
	}
	return E('div', { 'class': 'hf-mon-panel' }, [
		E('h4', {}, title),
		dl
	]);
}

function buildErrorBanner(msg) {
	return E('div', { 'class': 'hf-mon-banner hf-mon-banner--bad' }, [
		E('strong', {}, _('Ошибка загрузки') + ': '),
		String(msg || _('неизвестная ошибка'))
	]);
}

function buildControllerTable(controllers) {
	if (!controllers || !controllers.length)
		return '';

	var thead = E('tr', {}, [
		E('th', {}, _('Секция')),
		E('th', {}, _('Режим')),
		E('th', {}, _('Активный')),
		E('th', {}, _('Primary')),
		E('th', {}, _('Задержка')),
		E('th', {}, _('fail / recover'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < controllers.length; i++) {
		var c = controllers[i];
		tbody.appendChild(E('tr', {}, [
			E('td', {}, c.section || '-'),
			E('td', {}, c.mode || '-'),
			E('td', {}, E('span', { 'class': 'hf-mon-tag' }, c.active || '-')),
			E('td', {}, badge(c.primary_ok, 'OK', 'FAIL')),
			E('td', {}, c.primary_delay_ms ? (c.primary_delay_ms + ' ms') : '-'),
			E('td', {}, (c.fail_streak || 0) + ' / ' + (c.recover_streak || 0))
		]));
	}
	return E('div', { 'class': 'hf-mon-section' }, [
		E('h3', {}, _('Контроллеры по секциям')),
		E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody])
	]);
}

function buildFailoverPanels(data) {
	var fo = data && data.failover;
	var ctrl = data && data.controller && data.controller[0];
	var routeRows = [
		[_('Секция'), fo ? fo.section : '-'],
		[_('Политика'), fo ? fo.policy : '-'],
		[_('Описание'), fo ? policyHint(fo.policy) : '-'],
		[_('Selector'), fo && fo.selector_now ? E('span', { 'class': 'hf-mon-tag' }, fo.selector_now) : '-'],
		[_('URLTest'), fo && fo.urltest_now ? E('span', { 'class': 'hf-mon-tag' }, fo.urltest_now) : '-']
	];
	var ctrlRows = [
		[_('Режим'), ctrl ? ctrl.mode : '-'],
		[_('Primary probe'), ctrl ? badge(ctrl.primary_ok, 'OK', 'FAIL') : '-'],
		[_('Задержка primary'), ctrl && ctrl.primary_delay_ms ? (ctrl.primary_delay_ms + ' ms') : '-'],
		[_('Счётчик сбоев'), ctrl ? String(ctrl.fail_streak) : '-'],
		[_('Счётчик восстановления'), ctrl ? String(ctrl.recover_streak) : '-']
	];
	if (ctrl && ctrl.last_error)
		ctrlRows.push([_('Ошибка'), ctrl.last_error]);
	var paramRows = [];
	if (fo) {
		if (fo.check_interval)
			paramRows.push(['URLTest interval', fo.check_interval]);
		if (fo.tolerance)
			paramRows.push(['Tolerance', fo.tolerance + ' ms']);
		if (fo.idle_timeout)
			paramRows.push(['Idle timeout', fo.idle_timeout]);
		if (fo.testing_url)
			paramRows.push(['Probe URL', E('span', { 'class': 'hf-mon-tag' }, fo.testing_url)]);
	}
	var panels = E('div', {}, [
		E('div', { 'class': 'hf-mon-panels' }, [
			buildKvPanel(_('Активный маршрут'), routeRows),
			E('div', {}, [
				buildKvPanel(_('Контроллер failover'), ctrlRows),
				paramRows.length ? buildKvPanel(_('Параметры URLTest'), paramRows) : ''
			])
		]),
		buildControllerTable(data && data.controller)
	]);
	return panels;
}

function buildChannelsTable(channels, probed) {
	if (!channels || !channels.length)
		return E('p', { 'class': 'hf-mon-empty' },
			_('Каналы не найдены. Включите VPN+failover в секции маршрутизации.'));

	var maxMs = maxChannelDelay(channels);
	var thead = E('tr', {}, [
		E('th', {}, _('Статус')),
		E('th', {}, _('Канал')),
		E('th', {}, _('Тип')),
		E('th', {}, _('Задержка')),
		E('th', {}, _('Тренд')),
		E('th', {}, _('Роль'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < channels.length; i++) {
		var ch = channels[i];
		var rowCls = ch.selected ? 'hf-mon-row--active' : '';
		var role = ch.selected ? _('активен') : (ch.probed || probed ? _('резерв') : _('кэш'));
		tbody.appendChild(E('tr', { 'class': rowCls }, [
			E('td', {}, badge(ch.available, 'UP', 'DOWN')),
			E('td', {}, ch.display || ch.name),
			E('td', {}, ch.type || '-'),
			E('td', {}, latencyBar(ch.delay_ms || 0, maxMs)),
			E('td', {}, buildSparklineSVG(ch.name)),
			E('td', {}, role)
		]));
	}
	if (!probed)
		tbody.appendChild(E('tr', {}, [
			E('td', { 'colspan': '6', 'style': 'font-size:12px;opacity:.7;' },
				_('Данные из кэша Clash. Нажмите «Live probe» для актуальной проверки.'))
		]));

	return E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody]);
}

function buildHistoryTable(events) {
	if (!events || !events.length)
		return E('p', { 'class': 'hf-mon-empty' }, _('Событий failover пока не было.'));

	var list = events.slice().reverse();
	if (list.length > 15)
		list = list.slice(0, 15);

	var thead = E('tr', {}, [
		E('th', {}, _('Время')),
		E('th', {}, _('Секция')),
		E('th', {}, _('Переход')),
		E('th', {}, _('Причина'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < list.length; i++) {
		var ev = list[i];
		var from = ev.from || ev.From || '-';
		var to = ev.to || ev.To || '-';
		tbody.appendChild(E('tr', {}, [
			E('td', {}, formatEventTime(ev.time || ev.Time)),
			E('td', {}, ev.section || ev.Section || '-'),
			E('td', {}, E('span', { 'class': 'hf-mon-tag' }, from + ' → ' + to)),
			E('td', {}, ev.reason || ev.Reason || '-')
		]));
	}
	return E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody]);
}

function buildMetaLine(data) {
	var m = data && data.meta;
	if (!m)
		return '';
	var parts = [];
	if (m.core_version)
		parts.push('core ' + m.core_version);
	if (m.singbox_version)
		parts.push(m.singbox_version);
	if (m.uci_schema)
		parts.push('schema ' + m.uci_schema);
	if (!parts.length)
		return '';
	return E('p', { 'class': 'hint', 'style': 'margin:0 0 12px;' }, parts.join(' · '));
}

function buildDryRunSection(hints) {
	if (!hints || !hints.length)
		return '';
	var ul = E('ul', { 'style': 'margin:0;padding-left:18px;font-size:13px;' });
	for (var i = 0; i < hints.length; i++) {
		ul.appendChild(E('li', {}, [
			E('strong', {}, hints[i].section + ': '),
			hints[i].suggestion
		]));
	}
	return E('div', { 'class': 'hf-mon-section' }, [
		E('h3', {}, _('Dry-run (контроллер)')),
		ul
	]);
}

function buildMonitorDOM(data, channels, channelsProbed, history) {
	return E('div', {}, [
		buildMetaLine(data),
		buildSummaryBanner(data),
		buildMetricCards(data),
		buildDryRunSection(data && data.dry_run),
		buildFailoverPanels(data),
		E('div', { 'class': 'hf-mon-section' }, [
			E('h3', {}, _('Каналы и задержки')),
			buildChannelsTable(channels, channelsProbed)
		]),
		E('div', { 'class': 'hf-mon-section' }, [
			E('h3', {}, _('История переключений')),
			buildHistoryTable(history)
		])
	]);
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	healthBusy: false,
	_lastStatus: null,
	_lastHistory: null,
	_loadError: null,
	_channelData: null,
	_channelsProbed: false,
	_healthStarted: false,
	_pollCountdown: 5,
	_switchSection: '',
	_monRoot: null,
	_monUpdated: null,
	_switchSectionEl: null,
	_switchOutboundEl: null,

	load: function() {
		return Promise.all([
			callStatus().then(function(res) {
				return { ok: true, data: unwrapData(res) };
			}).catch(function(err) {
				return { ok: false, error: err };
			}),
			callHistory().then(function(res) {
				return { ok: true, data: unwrapData(res) };
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

		var content;
		if (this._loadError)
			content = buildErrorBanner(this._loadError);
		else
			content = buildMonitorDOM(
				this._lastStatus,
				this._channelData || (this._lastStatus && this._lastStatus.channels) || [],
				this._channelsProbed,
				this._lastHistory || []
			);

		emptyNode(root);
		root.appendChild(content);
		if (updated)
			updated.textContent = _('Обновлено') + ': ' + new Date().toLocaleTimeString() +
				' · ' + _('следующее через') + ' ' + self._pollCountdown + 's';
	},

	updateSwitchPanel: function() {
		var secEl = this._switchSectionEl;
		var outEl = this._switchOutboundEl;
		if (!secEl || !outEl)
			return;
		var data = this._lastStatus;
		var fo = data && data.failover;
		var section = (fo && fo.section) || this._switchSection || 'glob';
		this._switchSection = section;
		secEl.value = section;
		while (outEl.firstChild)
			outEl.removeChild(outEl.firstChild);
		var channels = this._channelData || (data && data.channels) || [];
		for (var i = 0; i < channels.length; i++) {
			var ch = channels[i];
			if (!ch.name)
				continue;
			outEl.appendChild(E('option', { 'value': ch.name }, ch.display || ch.name));
		}
	},

	applyLoadResults: function(results) {
		if (results[0] && results[0].ok) {
			this._lastStatus = results[0].data;
			this._loadError = null;
			var chans = this._lastStatus.channels || [];
			recordDelayHistory(chans);
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
	},

	refreshAll: function() {
		var self = this;
		return self.load().then(function(results) {
			self.applyLoadResults(results);
			self.renderMonitor();
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
		return callHealth().then(function(res) {
			var data = unwrapData(res);
			self._channelData = data && data.channels;
			self._channelsProbed = true;
			self._healthStarted = true;
			if (data && self._lastStatus) {
				if (data.channels) {
					self._lastStatus.channels = data.channels;
					recordDelayHistory(data.channels);
				}
				if (data.controller)
					self._lastStatus.controller = data.controller;
				if (data.failover)
					self._lastStatus.failover = data.failover;
			}
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

		self.applyLoadResults(loadResults || [{ ok: false }, { ok: false }]);

		var root = E('div', { 'id': 'hf-mon-root' });
		self._monRoot = root;
		self.renderMonitor();

		var box = E('div', { 'class': 'cbi-section hf-mon' }, [
			E('style', { 'type': 'text/css' }, HF_MON_CSS),
			E('h2', {}, _('Hybrid Failover: мониторинг')),
			E('p', { 'class': 'hint' },
				_('Сводка состояния, каналы failover, контроллер политики и журнал переключений. Обновление каждые 5 с.')),
			E('div', { 'class': 'hf-mon-toolbar' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'id': 'hf-btn-refresh',
					'click': ui.createHandlerFn(self, function() { return self.refreshAll(); })
				}, _('Обновить')),
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'id': 'hf-btn-probe',
					'click': ui.createHandlerFn(self, function() { return self.runHealthProbe(); })
				}, _('Live probe каналов')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return callCheckFakeip().then(function(res) {
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
						clearDelayHistory();
						ui.addNotification(null, E('p', {}, _('Графики задержек сброшены')), 'info');
						return Promise.resolve();
					})
				}, _('Сбросить графики')),
				E('span', { 'class': 'hf-mon-updated', 'id': 'hf-mon-updated' }, '')
			]),
			E('div', { 'class': 'hf-mon-section' }, [
				E('h3', {}, _('Ручное переключение канала')),
				E('div', { 'class': 'hf-mon-switch' }, [
					E('div', {}, [
						E('label', {}, _('Секция')),
						E('input', {
							'class': 'cbi-input-text',
							'id': 'hf-switch-section',
							'change': ui.createHandlerFn(self, function(ev) {
								self._switchSection = ev.target.value;
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
							if (!confirm(_('Переключить selector на ') + outbound.value + '?'))
								return Promise.resolve();
							return callSwitchProxy(section.value, outbound.value).then(function(res) {
								var ok = res && res.ok !== false && !(res.data && res.data.ok === false);
								ui.addNotification(null, E('p', {},
									ok ? _('Переключено') : (res.data && res.data.error) || _('Ошибка')),
									ok ? 'info' : 'danger');
								if (ok)
									return self.refreshAll();
							}).catch(function(err) {
								ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
							});
						})
					}, _('Переключить'))
				])
			]),
			root
		]);

		self._monUpdated = box.querySelector('#hf-mon-updated');
		self._switchSectionEl = document.getElementById('hf-switch-section');
		self._switchOutboundEl = document.getElementById('hf-switch-outbound');
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
