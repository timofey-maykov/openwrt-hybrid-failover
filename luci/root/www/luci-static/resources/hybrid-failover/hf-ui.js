'use strict';
'require baseclass';
'require rpc';
'require ui';

var HF_DELAY_MAX = 40;

var HF_CSS = [
	'.hf-mon { max-width: 1100px; margin: 0 auto 24px; color: var(--cbi-section-text-color, inherit); }',
	'.hf-mon-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 16px; }',
	'.hf-mon-toolbar .hf-mon-updated { opacity: 0.75; font-size: 12px; margin-left: auto; }',
	'.hf-mon-toolbar .btn:focus-visible { outline: 2px solid #2980b9; outline-offset: 2px; }',
	'.hf-mon-banner { padding: 10px 14px; border-radius: 8px; margin-bottom: 16px; font-size: 13px; }',
	'.hf-mon-banner--sticky { position: sticky; top: 0; z-index: 20; backdrop-filter: blur(6px); }',
	'.hf-mon-banner--ok { background: rgba(60,186,84,.12); border: 1px solid rgba(60,186,84,.4); }',
	'.hf-mon-banner--warn { background: rgba(240,173,78,.12); border: 1px solid rgba(240,173,78,.45); }',
	'.hf-mon-banner--bad { background: rgba(231,76,60,.12); border: 1px solid rgba(231,76,60,.45); }',
	'.hf-mon-banner__links { margin-top: 8px; display: flex; flex-wrap: wrap; gap: 10px; font-size: 12px; }',
	'.hf-mon-banner__links a { text-decoration: underline; }',
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
	'.hf-mon-table-wrap { overflow-x: auto; -webkit-overflow-scrolling: touch; margin-bottom: 8px; }',
	'.hf-mon-table { width: 100%; border-collapse: collapse; font-size: 13px; min-width: 520px; }',
	'.hf-mon-table th, .hf-mon-table td { padding: 8px 10px; text-align: left; border-bottom: 1px solid var(--border-color, rgba(127,127,127,.25)); }',
	'.hf-mon-table th { font-size: 11px; text-transform: uppercase; letter-spacing: .03em; opacity: .75; font-weight: 600; }',
	'.hf-mon-table tr.hf-mon-row--active { background: rgba(60,186,84,.08); }',
	'.hf-mon-badge { display: inline-block; padding: 2px 8px; border-radius: 999px; font-size: 11px; font-weight: 600; }',
	'.hf-mon-badge--ok { background: rgba(60,186,84,.2); color: #2d8a3e; }',
	'.hf-mon-badge--bad { background: rgba(231,76,60,.2); color: #c0392b; }',
	'.hf-mon-badge--warn { background: rgba(240,173,78,.25); color: #b8860b; }',
	'.hf-mon-badge--info { background: rgba(52,152,219,.2); color: #2980b9; }',
	'.hf-mon-chip { display: inline-block; padding: 2px 6px; border-radius: 4px; font-size: 11px; background: rgba(127,127,127,.15); margin-right: 4px; }',
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
	'.hf-mon-switch select { min-width: 200px; }',
	'.hf-mon-flow { font-size: 12px; opacity: .85; padding: 8px 12px; background: rgba(127,127,127,.08); border-radius: 6px; margin-bottom: 12px; font-family: ui-monospace, monospace; }',
	'.hf-mon-stepper { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin: 12px 0; }',
	'.hf-mon-stepper .hf-step-result { flex: 1 1 100%; font-size: 12px; padding: 10px; border-radius: 6px; background: var(--cbi-section-background-color, rgba(127,127,127,.06)); max-height: 160px; overflow: auto; white-space: pre-wrap; }',
	'.hf-mon-modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.45); z-index: 1000; display: flex; align-items: center; justify-content: center; padding: 16px; }',
	'.hf-mon-modal { background: var(--background-color, #fff); color: inherit; border-radius: 10px; max-width: 520px; width: 100%; padding: 18px; box-shadow: 0 8px 32px rgba(0,0,0,.2); }',
	'.hf-mon-modal--wide { max-width: min(920px, 96vw); }',
	'.hf-mon-modal h4 { margin: 0 0 12px; }',
	'.hf-mon-modal-body { overflow: hidden; }',
	'.hf-mon-modal .hf-mon-table-wrap { margin: 0; overflow: visible; }',
	'.hf-mon-modal .hf-mon-table { min-width: 0; width: 100%; table-layout: fixed; font-size: 12px; }',
	'.hf-mon-modal .hf-mon-table th, .hf-mon-modal .hf-mon-table td { padding: 6px 8px; word-break: break-word; overflow-wrap: anywhere; }',
	'.hf-mon-modal .hf-mon-table .hf-mon-col-ip { white-space: nowrap; }',
	'.hf-mon-modal .hf-mon-table .hf-mon-col-mac { font-size: 11px; font-family: ui-monospace, monospace; }',
	'.hf-mon-modal .hf-mon-table .hf-mon-col-lease { white-space: nowrap; }',
	'.hf-mon-modal .hf-mon-table .hf-mon-col-act { white-space: nowrap; text-align: right; width: 72px; }',
	'.hf-mon-modal .hf-mon-table .btn { padding: 4px 8px; font-size: 11px; white-space: nowrap; }',
	'.hf-mon-modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 14px; }',
	'.hf-mon-checklist { list-style: none; margin: 0; padding: 0; font-size: 13px; }',
	'.hf-mon-checklist li { padding: 6px 0; border-bottom: 1px solid var(--border-color, rgba(127,127,127,.2)); }',
	'@media (max-width: 720px) {',
	'  .hf-mon-card-grid-mobile .hf-mon-table { min-width: 0; }',
	'  .hf-mon-card-row { display: block; border: 1px solid var(--border-color, rgba(127,127,127,.25)); border-radius: 8px; padding: 10px; margin-bottom: 8px; }',
	'}'
].join('\n');

var rpcStatus = rpc.declare({ object: 'hybrid-failover', method: 'status' });
var rpcHealth = rpc.declare({ object: 'hybrid-failover', method: 'health' });
var rpcHistory = rpc.declare({ object: 'hybrid-failover', method: 'history' });
var rpcDelayHistory = rpc.declare({ object: 'hybrid-failover', method: 'delay_history' });
var rpcCheckFakeip = rpc.declare({ object: 'hybrid-failover', method: 'check_fakeip' });
var rpcExportHistory = rpc.declare({ object: 'hybrid-failover', method: 'export_history' });
var rpcSwitchProxy = rpc.declare({ object: 'hybrid-failover', method: 'switch_proxy', params: [ 'section', 'outbound' ] });
var rpcGlobalCheck = rpc.declare({ object: 'hybrid-failover', method: 'global_check' });
var rpcListClients = rpc.declare({ object: 'hybrid-failover', method: 'list_clients' });
var rpcDhcpLeases = rpc.declare({ object: 'hybrid-failover', method: 'dhcp_leases' });
var rpcMetrics = rpc.declare({ object: 'hybrid-failover', method: 'metrics' });

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

function badgeWarn(label) {
	return E('span', { 'class': 'hf-mon-badge hf-mon-badge--warn' }, label);
}

function badgeInfo(label) {
	return E('span', { 'class': 'hf-mon-badge hf-mon-badge--info' }, label);
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

function formatEventTime(raw) {
	if (!raw)
		return '-';
	var d = new Date(raw);
	if (isNaN(d.getTime()))
		return String(raw);
	return d.toLocaleString();
}

function formatRelativeTime(raw) {
	if (!raw)
		return '-';
	var d = new Date(raw);
	if (isNaN(d.getTime()))
		return String(raw);
	var sec = Math.floor((Date.now() - d.getTime()) / 1000);
	if (sec < 60)
		return sec + ' ' + _('с назад');
	if (sec < 3600)
		return Math.floor(sec / 60) + ' ' + _('мин назад');
	if (sec < 86400)
		return Math.floor(sec / 3600) + ' ' + _('ч назад');
	return Math.floor(sec / 86400) + ' ' + _('д назад');
}

function policyHint(policy) {
	switch (policy) {
	case 'outage-only':
		return _('VPN пока probe OK; при сбоях — резервы');
	case 'prefer-primary':
		return _('Предпочитать VPN; быстрый возврат');
	case 'fastest':
		return _('Выбор самого быстрого канала (urltest)');
	default:
		return policy || '-';
	}
}

function streakChip(streak, threshold, label) {
	if (!threshold || threshold <= 0)
		return String(streak || 0);
	return (streak || 0) + '/' + threshold + ' ' + label;
}

function recordDelayHistoryLocal(channels) {
	if (!channels || !window.localStorage)
		return;
	for (var i = 0; i < channels.length; i++) {
		var ch = channels[i];
		if (!ch.name || !ch.delay_ms)
			continue;
		var key = 'hf_delay_' + ch.name;
		var list = [];
		try { list = JSON.parse(localStorage.getItem(key) || '[]'); } catch (e) { list = []; }
		list.push({ t: Date.now(), d: ch.delay_ms });
		if (list.length > HF_DELAY_MAX)
			list = list.slice(list.length - HF_DELAY_MAX);
		localStorage.setItem(key, JSON.stringify(list));
	}
}

function clearDelayHistoryLocal() {
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

function delayPointsFromServer(serverData, tag) {
	if (!serverData || !tag)
		return null;
	var channels = serverData.channels || serverData;
	if (Array.isArray(channels)) {
		for (var i = 0; i < channels.length; i++) {
			if (channels[i].tag === tag || channels[i].name === tag)
				return channels[i].points || channels[i].samples;
		}
	}
	if (serverData[tag])
		return serverData[tag];
	return null;
}

function buildSparklineSVG(name, serverDelayData) {
	var points = [];
	var serverPts = delayPointsFromServer(serverDelayData, name);
	if (serverPts && serverPts.length) {
		for (var s = 0; s < serverPts.length; s++) {
			var pt = serverPts[s];
			points.push({ d: pt.delay_ms != null ? pt.delay_ms : pt.d, t: pt.time || pt.t });
		}
	}
	if (!points.length) {
		try {
			points = JSON.parse(localStorage.getItem('hf_delay_' + name) || '[]');
		} catch (e) { points = []; }
	}
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

function wrapTable(tableEl) {
	return E('div', { 'class': 'hf-mon-table-wrap' }, [tableEl]);
}

function buildErrorBanner(msg) {
	return E('div', { 'class': 'hf-mon-banner hf-mon-banner--bad hf-mon-banner--sticky' }, [
		E('strong', {}, _('Ошибка') + ': '),
		String(msg || _('неизвестная ошибка'))
	]);
}

function buildSummaryBanner(data, opts) {
	opts = opts || {};
	var state = overallState(data);
	var title, cls, text;
	if (state === 'up') {
		cls = 'hf-mon-banner hf-mon-banner--ok hf-mon-banner--sticky';
		title = _('Маршрутизация активна');
		text = _('Все критичные компоненты работают.');
	} else if (state === 'degraded') {
		cls = 'hf-mon-banner hf-mon-banner--warn hf-mon-banner--sticky';
		title = _('Частичная деградация');
		text = _('Сервис работает, но есть предупреждения.');
	} else if (state === 'down') {
		cls = 'hf-mon-banner hf-mon-banner--bad hf-mon-banner--sticky';
		title = _('Маршрутизация неактивна');
		text = _('sing-box, nft или Clash API недоступны.');
	} else {
		cls = 'hf-mon-banner hf-mon-banner--sticky';
		title = _('Нет данных');
		text = _('Не удалось получить статус с роутера.');
	}
	var children = [E('strong', {}, title + ': '), text];
	if (data && data.active_outbound) {
		var disp = data.active_outbound;
		if (data.channels) {
			for (var i = 0; i < data.channels.length; i++) {
				if (data.channels[i].name === data.active_outbound && data.channels[i].display) {
					disp = data.channels[i].display;
					break;
				}
			}
		}
		children.push(E('span', { 'class': 'hf-mon-tag', 'style': 'display:block;margin-top:6px;' },
			_('Активный канал') + ': ' + disp));
	}
	if (data && data.errors && data.errors.length)
		children.push(E('div', { 'style': 'margin-top:8px;color:#c0392b;' }, data.errors.join(' · ')));
	var ctrlErr = opts.primaryError;
	if (ctrlErr)
		children.push(E('div', { 'style': 'margin-top:6px;color:#c0392b;' }, _('Primary') + ': ' + ctrlErr));
	var links = E('div', { 'class': 'hf-mon-banner__links' }, [
		E('a', { 'href': L.url('admin/services/hybrid-failover/diagnostics') }, _('Диагностика')),
		E('a', { 'href': L.url('admin/services/hybrid-failover/routing') }, _('Настройки')),
		E('a', { 'href': '#hf-switch-block' }, _('Переключить канал'))
	]);
	children.push(links);
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
	var policyVal = '-';
	if (data && data.failover && data.failover.policy)
		policyVal = data.failover.policy;
	return E('div', { 'class': 'hf-mon-grid' }, [
		card('sing-box', data && data.singbox_running ? _('работает') : _('остановлен'),
			data && data.singbox_running ? 'ok' : 'bad'),
		card('nft / tproxy', data && data.nft_ok ? 'OK' : _('ошибка'),
			data && data.nft_ok ? 'ok' : 'bad'),
		card('Clash API', data && data.clash_ok ? 'OK' : _('недоступен'),
			data && data.clash_ok ? 'ok' : 'bad'),
		card('fakeip DNS', fakeipVal, fakeipState),
		card(_('Политика'), policyVal, 'info'),
		card(_('Активный outbound'), (data && data.active_outbound) || '-',
			data && data.active_outbound ? 'ok' : 'neutral')
	]);
}

function buildOutageFlowDiagram(mode) {
	if (mode !== 'primary' && mode !== 'backup')
		return '';
	var activeOnPrimary = mode === 'primary';
	return E('div', { 'class': 'hf-mon-flow' },
		'[Primary VPN] ──fail──► [URLTest резервы] ──recover──► [Primary VPN]\n' +
		(activeOnPrimary ? '     ▲ active (primary)' : '                    ▲ active (backup)'));
}

function buildControllerTable(controllers, sectionFilter, dryRun) {
	if (!controllers || !controllers.length)
		return '';
	var thead = E('tr', {}, [
		E('th', {}, _('Секция')),
		E('th', {}, _('Policy')),
		E('th', {}, _('Режим')),
		E('th', {}, _('Активный')),
		E('th', {}, _('Primary')),
		E('th', {}, _('Probe')),
		E('th', {}, _('Счётчики'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < controllers.length; i++) {
		var c = controllers[i];
		if (sectionFilter && c.section !== sectionFilter)
			continue;
		var failTh = 2, recTh = 2;
		var streakText = streakChip(c.fail_streak, failTh, _('fail')) + ' · ' +
			streakChip(c.recover_streak, recTh, _('recover'));
		var probeInfo = [];
		if (c.last_probe_at)
			probeInfo.push(formatRelativeTime(c.last_probe_at));
		if (c.last_error)
			probeInfo.push(c.last_error);
		tbody.appendChild(E('tr', {}, [
			E('td', {}, c.section || '-'),
			E('td', {}, c.policy ? badgeInfo(c.policy) : '-'),
			E('td', {}, c.mode || '-'),
			E('td', {}, E('span', { 'class': 'hf-mon-tag' }, c.active || '-')),
			E('td', {}, badge(c.primary_ok, 'OK', 'FAIL')),
			E('td', { 'title': c.last_error || '' }, probeInfo.join(' · ') || '-'),
			E('td', {}, streakText)
		]));
	}
	var hintEl = '';
	if (dryRun && dryRun.length) {
		var ul = E('ul', { 'style': 'margin:8px 0 0;padding-left:18px;font-size:12px;' });
		for (var h = 0; h < dryRun.length; h++) {
			if (sectionFilter && dryRun[h].section !== sectionFilter)
				continue;
			ul.appendChild(E('li', {}, [
				E('strong', {}, dryRun[h].section + ': '),
				dryRun[h].suggestion
			]));
		}
		hintEl = ul;
	}
	return E('div', { 'class': 'hf-mon-section' }, [
		E('h3', {}, _('Контроллер failover')),
		wrapTable(E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody])),
		hintEl
	]);
}

function buildFailoverPanels(data, sectionFilter) {
	var fo = data && data.failover;
	var ctrl = null;
	if (data && data.controller) {
		for (var ci = 0; ci < data.controller.length; ci++) {
			if (sectionFilter && data.controller[ci].section === sectionFilter) {
				ctrl = data.controller[ci];
				break;
			}
			if (!sectionFilter && fo && data.controller[ci].section === fo.section) {
				ctrl = data.controller[ci];
				break;
			}
		}
		if (!ctrl && data.controller.length)
			ctrl = data.controller[0];
	}
	var routeRows = [
		[_('Секция'), (ctrl && ctrl.section) || (fo && fo.section) || sectionFilter || '-'],
		[_('Политика'), fo ? fo.policy : (ctrl && ctrl.policy) || '-'],
		[_('Описание'), fo ? policyHint(fo.policy) : policyHint(ctrl && ctrl.policy)],
		[_('Selector'), fo && fo.selector_now ? E('span', { 'class': 'hf-mon-tag' }, fo.selector_now) : '-'],
		[_('URLTest'), fo && fo.urltest_now ? E('span', { 'class': 'hf-mon-tag' }, fo.urltest_now) : '-']
	];
	var ctrlRows = [
		[_('Режим'), ctrl ? ctrl.mode : '-'],
		[_('Primary probe'), ctrl ? badge(ctrl.primary_ok, 'OK', 'FAIL') : '-'],
		[_('Задержка primary'), ctrl && ctrl.primary_delay_ms ? (ctrl.primary_delay_ms + ' ms') : '-'],
		[_('Последний probe'), ctrl && ctrl.last_probe_at ? formatRelativeTime(ctrl.last_probe_at) : '-'],
		[_('На канале с'), ctrl && ctrl.active_since ? formatRelativeTime(ctrl.active_since) : '-'],
		[_('Последнее переключение'), ctrl && ctrl.last_switch_at ? formatRelativeTime(ctrl.last_switch_at) : '-']
	];
	if (ctrl && ctrl.last_error)
		ctrlRows.push([_('Ошибка probe'), E('span', { 'style': 'color:#c0392b;' }, ctrl.last_error)]);
	var paramRows = [];
	if (fo) {
		if (fo.check_interval)
			paramRows.push([_('URLTest interval'), fo.check_interval]);
		if (fo.tolerance)
			paramRows.push([_('Tolerance'), fo.tolerance + ' ms']);
		if (fo.idle_timeout)
			paramRows.push([_('Idle timeout'), fo.idle_timeout]);
		if (fo.testing_url)
			paramRows.push([_('Probe URL'), E('span', { 'class': 'hf-mon-tag' }, fo.testing_url)]);
	}
	return E('div', {}, [
		ctrl ? buildOutageFlowDiagram(ctrl.mode) : '',
		E('div', { 'class': 'hf-mon-panels' }, [
			buildKvPanel(_('Маршрут'), routeRows),
			E('div', {}, [
				buildKvPanel(_('Контроллер'), ctrlRows),
				paramRows.length ? buildKvPanel(_('URLTest'), paramRows) : ''
			])
		]),
		buildControllerTable(data && data.controller, sectionFilter, data && data.dry_run)
	]);
}

function buildChannelsTable(channels, probed, serverDelayData) {
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
		var statusCell = badge(ch.available, 'UP', 'DOWN');
		if (ch.detail)
			statusCell = E('div', {}, [statusCell, E('div', { 'style': 'font-size:11px;opacity:.75;margin-top:2px;' }, ch.detail)]);
		tbody.appendChild(E('tr', { 'class': rowCls }, [
			E('td', {}, statusCell),
			E('td', {}, [
				E('div', {}, ch.display || ch.name),
				E('div', { 'class': 'hf-mon-tag', 'style': 'opacity:.7;' }, ch.name)
			]),
			E('td', {}, ch.type || '-'),
			E('td', {}, latencyBar(ch.delay_ms || 0, maxMs)),
			E('td', {}, buildSparklineSVG(ch.name, serverDelayData)),
			E('td', {}, role)
		]));
	}
	if (!probed)
		tbody.appendChild(E('tr', {}, [
			E('td', { 'colspan': '6', 'style': 'font-size:12px;opacity:.7;' },
				_('Данные из кэша Clash. Нажмите «Live probe» для актуальной проверки.'))
		]));

	return wrapTable(E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody]));
}

function buildHistoryTable(events, sectionFilter, limit) {
	if (!events || !events.length)
		return E('p', { 'class': 'hf-mon-empty' }, _('Событий failover пока не было.'));

	var list = events.slice().reverse();
	if (sectionFilter) {
		list = list.filter(function(ev) {
			return (ev.section || ev.Section) === sectionFilter;
		});
	}
	limit = limit || 15;
	if (list.length > limit)
		list = list.slice(0, limit);

	var thead = E('tr', {}, [
		E('th', {}, _('Время')),
		E('th', {}, _('Секция')),
		E('th', {}, _('Переход')),
		E('th', {}, _('Причина')),
		E('th', {}, _('Policy')),
		E('th', {}, _('Probe'))
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
			E('td', {}, ev.reason || ev.Reason || '-'),
			E('td', {}, ev.policy || ev.Policy || '-'),
			E('td', {}, ev.probe_ms ? (ev.probe_ms + ' ms') : '-')
		]));
	}
	return wrapTable(E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody]));
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

function sectionOptions(controllers, fallback) {
	var opts = [];
	var seen = {};
	if (controllers) {
		for (var i = 0; i < controllers.length; i++) {
			var s = controllers[i].section;
			if (s && !seen[s]) {
				seen[s] = true;
				opts.push(s);
			}
		}
	}
	if (fallback && !seen[fallback])
		opts.unshift(fallback);
	if (!opts.length)
		opts.push('glob');
	return opts;
}

function showModal(title, bodyNodes, onConfirm, opts) {
	opts = opts || {};
	var modalClass = 'hf-mon-modal' + (opts.wide ? ' hf-mon-modal--wide' : '');
	var backdrop = E('div', { 'class': 'hf-mon-modal-backdrop' });
	var modal = E('div', { 'class': modalClass }, [
		E('h4', {}, title),
		E('div', { 'class': 'hf-mon-modal-body' }, bodyNodes),
		E('div', { 'class': 'hf-mon-modal-actions' }, [
			E('button', {
				'class': 'btn cbi-button cbi-button-neutral',
				'click': function() { document.body.removeChild(backdrop); }
			}, _('Отмена')),
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': function() {
					document.body.removeChild(backdrop);
					if (onConfirm)
						onConfirm();
				}
			}, _('OK'))
		])
	]);
	backdrop.appendChild(modal);
	backdrop.addEventListener('click', function(ev) {
		if (ev.target === backdrop)
			document.body.removeChild(backdrop);
	});
	document.body.appendChild(backdrop);
}

function parseChecklist(data) {
	var items = [];
	if (!data)
		return items;
	var d = data.data || data;
	if (typeof d === 'string') {
		d.split('\n').forEach(function(line) {
			line = line.trim();
			if (line)
				items.push({ ok: !/^fail|error/i.test(line), text: line });
		});
		return items;
	}
	if (d.errors && Array.isArray(d.errors)) {
		d.errors.forEach(function(e) { items.push({ ok: false, text: String(e) }); });
	}
	if (d.ok === true || d.ok === false)
		items.unshift({ ok: !!d.ok, text: d.message || (d.ok ? 'OK' : 'FAIL') });
	if (d.singbox_running != null)
		items.push({ ok: !!d.singbox_running, text: 'sing-box: ' + (d.singbox_running ? 'running' : 'stopped') });
	if (d.nft_ok != null)
		items.push({ ok: !!d.nft_ok, text: 'nft: ' + (d.nft_ok ? 'OK' : 'FAIL') });
	if (d.fakeip_ok != null)
		items.push({ ok: !!d.fakeip_ok, text: 'fakeip: ' + (d.fakeip_ok ? 'OK' : 'FAIL') });
	if (d.clash_ok != null)
		items.push({ ok: !!d.clash_ok, text: 'Clash API: ' + (d.clash_ok ? 'OK' : 'FAIL') });
	return items;
}

function renderChecklist(items) {
	if (!items.length)
		return E('p', { 'class': 'hf-mon-empty' }, '-');
	var ul = E('ul', { 'class': 'hf-mon-checklist' });
	items.forEach(function(it) {
		ul.appendChild(E('li', {}, [
			it.ok ? '✓ ' : '✗ ',
			it.text
		]));
	});
	return ul;
}

function notifyRpcResult(title, res) {
	var text = (res && res.output) ? res.output :
		(res && res.ok === false) ? _('Ошибка') :
		(res && res.data) ? JSON.stringify(res.data, null, 2) :
		_('Готово');
	ui.addNotification(null, E('div', [
		E('strong', {}, title),
		E('pre', { 'style': 'white-space:pre-wrap;margin:8px 0 0;' }, text)
	]), res && res.ok === false ? 'danger' : 'info');
}

function injectStyles(parent) {
	parent.appendChild(E('style', { 'type': 'text/css' }, HF_CSS));
}

return baseclass.extend({
	HF_CSS: HF_CSS,
	HF_DELAY_MAX: HF_DELAY_MAX,
	rpc: {
		status: rpcStatus,
		health: rpcHealth,
		history: rpcHistory,
		delayHistory: rpcDelayHistory,
		checkFakeip: rpcCheckFakeip,
		exportHistory: rpcExportHistory,
		switchProxy: rpcSwitchProxy,
		globalCheck: rpcGlobalCheck,
		listClients: rpcListClients,
		dhcpLeases: rpcDhcpLeases,
		metrics: rpcMetrics
	},
	emptyNode: emptyNode,
	unwrapData: unwrapData,
	overallState: overallState,
	badge: badge,
	badgeWarn: badgeWarn,
	badgeInfo: badgeInfo,
	card: card,
	latencyBar: latencyBar,
	formatEventTime: formatEventTime,
	formatRelativeTime: formatRelativeTime,
	policyHint: policyHint,
	streakChip: streakChip,
	recordDelayHistoryLocal: recordDelayHistoryLocal,
	clearDelayHistoryLocal: clearDelayHistoryLocal,
	buildSparklineSVG: buildSparklineSVG,
	maxChannelDelay: maxChannelDelay,
	buildKvPanel: buildKvPanel,
	wrapTable: wrapTable,
	buildErrorBanner: buildErrorBanner,
	buildSummaryBanner: buildSummaryBanner,
	buildMetricCards: buildMetricCards,
	buildFailoverPanels: buildFailoverPanels,
	buildChannelsTable: buildChannelsTable,
	buildHistoryTable: buildHistoryTable,
	buildMetaLine: buildMetaLine,
	buildControllerTable: buildControllerTable,
	sectionOptions: sectionOptions,
	showModal: showModal,
	parseChecklist: parseChecklist,
	renderChecklist: renderChecklist,
	notifyRpcResult: notifyRpcResult,
	injectStyles: injectStyles
});
