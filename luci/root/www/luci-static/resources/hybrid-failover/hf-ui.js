'use strict';
'require baseclass';
'require rpc';
'require ui';

var HF_DELAY_MAX = 40;

var HF_CSS = [
	'.hf-mon { max-width: 1280px; margin: 0 auto 28px; color: var(--cbi-section-text-color, inherit); }',
	'.hf-ent-top { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px 20px; margin-bottom: 18px; padding-bottom: 14px; border-bottom: 1px solid var(--border-color, rgba(127,127,127,.25)); }',
	'.hf-ent-top__brand h2 { margin: 0 0 6px; font-size: 1.35rem; font-weight: 700; letter-spacing: -.02em; }',
	'.hf-ent-top__meta { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }',
	'.hf-ent-top__actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-left: auto; }',
	'.hf-ent-pill { display: inline-flex; align-items: center; gap: 6px; padding: 3px 10px; border-radius: 999px; font-size: 11px; font-weight: 600; letter-spacing: .02em; text-transform: uppercase; }',
	'.hf-ent-pill--ok { background: rgba(60,186,84,.15); color: #2d8a3e; }',
	'.hf-ent-pill--warn { background: rgba(240,173,78,.18); color: #9a6700; }',
	'.hf-ent-pill--bad { background: rgba(231,76,60,.15); color: #c0392b; }',
	'.hf-ent-pill--neutral { background: rgba(127,127,127,.12); opacity: .85; }',
	'.hf-ent-pill__dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }',
	'.hf-ent-hero { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 14px 20px; padding: 16px 20px; border-radius: 10px; margin-bottom: 16px; border: 1px solid var(--border-color, rgba(127,127,127,.3)); background: var(--cbi-section-background-color, rgba(127,127,127,.04)); }',
	'.hf-ent-hero--ok { border-left: 4px solid #3cba54; }',
	'.hf-ent-hero--warn { border-left: 4px solid #f0ad4e; }',
	'.hf-ent-hero--bad { border-left: 4px solid #e74c3c; }',
	'.hf-ent-hero__main { flex: 1 1 260px; min-width: 0; }',
	'.hf-ent-hero__title { margin: 0 0 4px; font-size: 1.05rem; font-weight: 700; }',
	'.hf-ent-hero__sub { margin: 0; font-size: 13px; opacity: .85; line-height: 1.45; }',
	'.hf-ent-hero__active { margin-top: 10px; font-size: 13px; }',
	'.hf-ent-hero__links { display: flex; flex-wrap: wrap; gap: 12px; font-size: 12px; align-self: center; }',
	'.hf-ent-hero__links a { text-decoration: none; padding: 6px 12px; border-radius: 6px; border: 1px solid var(--border-color, rgba(127,127,127,.35)); }',
	'.hf-ent-hero__links a:hover { background: rgba(127,127,127,.08); }',
	'.hf-ent-tabs { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 16px; border-bottom: 1px solid var(--border-color, rgba(127,127,127,.25)); }',
	'.hf-ent-tab { appearance: none; background: transparent; border: none; border-bottom: 2px solid transparent; margin-bottom: -1px; padding: 10px 14px; font-size: 13px; font-weight: 500; cursor: pointer; color: inherit; opacity: .75; }',
	'.hf-ent-tab:hover { opacity: 1; }',
	'.hf-ent-tab--active { opacity: 1; font-weight: 700; border-bottom-color: #2980b9; color: #2980b9; }',
	'.hf-ent-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; align-items: start; }',
	'.hf-ent-sidebar { display: flex; flex-direction: column; gap: 12px; position: sticky; top: 8px; }',
	'.hf-ent-card { border: 1px solid var(--border-color, rgba(127,127,127,.3)); border-radius: 10px; padding: 14px 16px; background: var(--cbi-section-background-color, rgba(127,127,127,.04)); }',
	'.hf-ent-card__title { margin: 0 0 12px; font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; opacity: .7; }',
	'.hf-ent-tools { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }',
	'.hf-ent-tools .btn { width: 100%; text-align: left; justify-content: flex-start; }',
	'.hf-ent-channels { margin-bottom: 16px; }',
	'.hf-ent-channels__summary { margin: 0 0 12px; font-size: 13px; opacity: .85; }',
	'.hf-ent-channel-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 10px; }',
	'.hf-ent-channel-card { border: 1px solid var(--border-color, rgba(127,127,127,.3)); border-radius: 10px; padding: 12px 14px; background: var(--cbi-section-background-color, rgba(127,127,127,.03)); }',
	'.hf-ent-channel-card--up { border-left: 3px solid #3cba54; }',
	'.hf-ent-channel-card--down { border-left: 3px solid #e74c3c; }',
	'.hf-ent-channel-card--unknown { border-left: 3px solid #f0ad4e; }',
	'.hf-ent-channel-card--active { box-shadow: inset 0 0 0 1px rgba(60,186,84,.35); background: rgba(60,186,84,.06); }',
	'.hf-ent-channel-card__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }',
	'.hf-ent-channel-card__role { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; opacity: .7; }',
	'.hf-ent-channel-card__name { font-size: 13px; font-weight: 600; line-height: 1.35; margin-bottom: 6px; word-break: break-word; }',
	'.hf-ent-channel-card__meta { font-size: 12px; opacity: .75; }',
	'.hf-ent-channel-card__flag { margin-top: 8px; font-size: 11px; font-weight: 700; color: #2d8a3e; text-transform: uppercase; }',
	'.hf-ent-channel-card__err { margin-top: 8px; font-size: 11px; color: #c0392b; line-height: 1.35; }',
	'.hf-ent-section-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; }',
	'.hf-ent-section-head h3 { margin: 0; font-size: 14px; font-weight: 700; }',
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
	'.hf-mon-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; margin-bottom: 18px; }',
	'@media (max-width: 1100px) { .hf-mon-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } .hf-ent-layout { grid-template-columns: 1fr; } .hf-ent-sidebar { position: static; } }',
	'@media (max-width: 640px) { .hf-mon-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }',
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
	'.hf-mon-table tbody tr:nth-child(even) { background: rgba(127,127,127,.03); }',
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

function isNativeEngine(data) {
	return !!(data && data.engine_mode === 'native');
}

function proxyRunning(data) {
	if (!data)
		return false;
	if (data.engine_running != null)
		return !!data.engine_running;
	return !!data.singbox_running;
}

function controlOk(data) {
	if (!data)
		return false;
	if (isNativeEngine(data))
		return proxyRunning(data);
	return !!data.clash_ok;
}

function controllerForSection(data, section) {
	if (!data || !data.controller)
		return null;
	for (var i = 0; i < data.controller.length; i++) {
		if (!section || data.controller[i].section === section)
			return data.controller[i];
	}
	return data.controller[0] || null;
}

function activeOutboundTag(data, section) {
	var ctrl = controllerForSection(data, section);
	if (ctrl && ctrl.active)
		return ctrl.active;
	if (data && data.failover && data.failover.selector_now)
		return data.failover.selector_now;
	return (data && data.active_outbound) || '';
}

function activeOutboundDisplay(data, section) {
	var tag = activeOutboundTag(data, section);
	if (!tag)
		return '';
	if (data && data.channels) {
		for (var i = 0; i < data.channels.length; i++) {
			if (data.channels[i].name === tag && data.channels[i].display)
				return data.channels[i].display;
		}
	}
	return tag;
}

function visibleErrors(data) {
	if (!data || !Array.isArray(data.errors))
		return [];
	if (!isNativeEngine(data))
		return data.errors;
	return data.errors.filter(function(e) {
		var s = String(e);
		return s.indexOf('clash') === -1 && s.indexOf('9090') === -1 && s.indexOf('channels:') === -1;
	});
}

function overallState(data) {
	if (!data || typeof data !== 'object')
		return 'unknown';
	var critical = proxyRunning(data) && data.nft_ok && controlOk(data);
	if (!critical)
		return 'down';
	if (data.fakeip_ok === false)
		return 'degraded';
	var errs = visibleErrors(data);
	if (errs.length)
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

function channelKind(ch) {
	if (!ch)
		return 'reserve';
	if (ch.type === 'direct' || /-awg-out$/.test(ch.name || ''))
		return 'primary';
	if (ch.type === 'urltest')
		return 'urltest';
	return 'reserve';
}

function channelAliveState(ch, probed, ctrl) {
	var kind = channelKind(ch);
	if (kind === 'primary' && ctrl) {
		if (ctrl.primary_ok === true)
			return 'up';
		if (ctrl.primary_ok === false)
			return 'down';
	}
	if (ch.available === true || (ch.delay_ms && ch.delay_ms > 0))
		return 'up';
	if (probed && ch.available === false && !(ch.delay_ms && ch.delay_ms > 0))
		return 'down';
	if (ch.detail && String(ch.detail).indexOf('no delay data') !== -1)
		return 'unknown';
	return ch.available === false ? 'down' : 'unknown';
}

function channelStatusBadge(ch, probed, ctrl) {
	var st = channelAliveState(ch, probed, ctrl);
	if (st === 'up')
		return badge(true, 'UP', 'DOWN');
	if (st === 'down')
		return badge(false, 'UP', 'DOWN');
	return badgeWarn(_('н/д'));
}

function channelRoleLabel(ch, ctrl) {
	var kind = channelKind(ch);
	if (kind === 'primary')
		return _('Primary VPN');
	if (kind === 'urltest')
		return _('URLTest группа');
	if (ch.selected || (ctrl && ctrl.urltest_member === ch.name))
		return _('активный резерв');
	return _('резерв');
}

function channelsReserveSummary(channels, data, probed) {
	if (!channels || !channels.length)
		return '';
	var ctrl = controllerForSection(data, data && data.failover && data.failover.section);
	var alive = 0, total = 0;
	for (var i = 0; i < channels.length; i++) {
		if (channelKind(channels[i]) !== 'reserve')
			continue;
		total++;
		if (channelAliveState(channels[i], probed, ctrl) === 'up')
			alive++;
	}
	if (!total)
		return '';
	return alive + '/' + total + ' ' + _('живы');
}

function buildChannelOverviewCard(ch, probed, ctrl) {
	var kind = channelKind(ch);
	var alive = channelAliveState(ch, probed, ctrl);
	var cardCls = 'hf-ent-channel-card hf-ent-channel-card--' + alive;
	if (ch.selected && kind === 'reserve')
		cardCls += ' hf-ent-channel-card--active';
	var metaParts = [];
	if (ch.type)
		metaParts.push(ch.type);
	if (ch.delay_ms && ch.delay_ms > 0)
		metaParts.push(ch.delay_ms + ' ms');
	var body = [
		E('div', { 'class': 'hf-ent-channel-card__head' }, [
			E('span', { 'class': 'hf-ent-channel-card__role' }, channelRoleLabel(ch, ctrl)),
			channelStatusBadge(ch, probed, ctrl)
		]),
		E('div', { 'class': 'hf-ent-channel-card__name' }, ch.display || ch.name),
		E('div', { 'class': 'hf-ent-channel-card__meta' }, metaParts.join(' · ') || '-')
	];
	if (ch.selected && kind === 'reserve')
		body.push(E('div', { 'class': 'hf-ent-channel-card__flag' }, _('активный резерв')));
	if (kind === 'primary' && ctrl && ctrl.last_error && ctrl.primary_ok === false)
		body.push(E('div', { 'class': 'hf-ent-channel-card__err' }, ctrl.last_error));
	else if (ch.detail && alive === 'unknown')
		body.push(E('div', { 'class': 'hf-ent-channel-card__err', 'style': 'color:#9a6700;' }, ch.detail));
	return E('div', { 'class': cardCls }, body);
}

function buildChannelsOverview(channels, data, probed, opts) {
	opts = opts || {};
	var ctrl = controllerForSection(data, opts.section);
	if (!channels || !channels.length) {
		return E('div', { 'class': 'hf-ent-card hf-ent-channels' }, [
			E('div', { 'class': 'hf-ent-section-head' }, [
				E('h3', {}, _('Каналы failover'))
			]),
			E('p', { 'class': 'hf-mon-empty' },
				_('Каналы не найдены. Включите VPN+failover в секции маршрутизации.'))
		]);
	}

	var primary = [], reserves = [];
	for (var i = 0; i < channels.length; i++) {
		var kind = channelKind(channels[i]);
		if (kind === 'urltest')
			continue;
		if (kind === 'primary')
			primary.push(channels[i]);
		else
			reserves.push(channels[i]);
	}

	var summaryParts = [];
	var reserveSummary = channelsReserveSummary(channels, data, probed);
	if (reserveSummary)
		summaryParts.push(_('Резервы') + ': ' + reserveSummary);
	if (ctrl && ctrl.mode)
		summaryParts.push(_('Режим') + ': ' + ctrl.mode);
	if (ctrl && ctrl.urltest_member)
		summaryParts.push(_('URLTest member') + ': ' + ctrl.urltest_member);
	else if (ctrl && ctrl.active && ctrl.mode === 'backup')
		summaryParts.push(_('Активный') + ': ' + activeOutboundDisplay(data, opts.section));

	var headChildren = [E('h3', {}, _('Каналы failover'))];
	if (typeof opts.onProbe === 'function') {
		headChildren.push(E('button', {
			'class': 'btn cbi-button cbi-button-save',
			'id': 'hf-btn-probe-overview',
			'click': opts.onProbe
		}, _('Live probe')));
	}

	var grid = E('div', { 'class': 'hf-ent-channel-grid' });
	for (var p = 0; p < primary.length; p++)
		grid.appendChild(buildChannelOverviewCard(primary[p], probed, ctrl));
	for (var r = 0; r < reserves.length; r++)
		grid.appendChild(buildChannelOverviewCard(reserves[r], probed, ctrl));

	var footer = '';
	if (!probed) {
		footer = E('p', { 'class': 'hint', 'style': 'margin:10px 0 0;font-size:12px;' },
			isNativeEngine(data)
				? _('Статус резервов из engine. Live probe обновит проверку всех каналов.')
				: _('Статус из кэша Clash. Live probe обновит проверку всех каналов.'));
	}

	return E('div', { 'class': 'hf-ent-card hf-ent-channels' }, [
		E('div', { 'class': 'hf-ent-section-head' }, headChildren),
		summaryParts.length
			? E('p', { 'class': 'hf-ent-channels__summary' }, summaryParts.join(' · '))
			: '',
		grid,
		footer
	]);
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

function buildStatusPill(state) {
	var label, cls;
	switch (state) {
	case 'up':
		label = _('В норме');
		cls = 'hf-ent-pill hf-ent-pill--ok';
		break;
	case 'degraded':
		label = _('Деградация');
		cls = 'hf-ent-pill hf-ent-pill--warn';
		break;
	case 'down':
		label = _('Недоступно');
		cls = 'hf-ent-pill hf-ent-pill--bad';
		break;
	default:
		label = _('Неизвестно');
		cls = 'hf-ent-pill hf-ent-pill--neutral';
	}
	return E('span', { 'class': cls }, [
		E('span', { 'class': 'hf-ent-pill__dot' }),
		label
	]);
}

function buildEnterpriseMeta(data) {
	var m = data && data.meta;
	var parts = [];
	if (m && m.core_version)
		parts.push('v' + m.core_version);
	if (data && data.engine_mode)
		parts.push(String(data.engine_mode));
	if (m && m.uci_schema)
		parts.push('schema ' + m.uci_schema);
	if (!parts.length)
		return E('span', { 'class': 'hint', 'style': 'font-size:12px;' }, '-');
	return E('span', { 'class': 'hint', 'style': 'font-size:12px;' }, parts.join(' · '));
}

function buildStatusHero(data, opts) {
	opts = opts || {};
	var state = overallState(data);
	var heroCls = 'hf-ent-hero hf-ent-hero--' +
		(state === 'up' ? 'ok' : state === 'degraded' ? 'warn' : state === 'down' ? 'bad' : 'neutral');
	var title, sub;
	if (state === 'up') {
		title = _('Маршрутизация активна');
		sub = _('Критичные компоненты в норме.');
	} else if (state === 'degraded') {
		title = _('Частичная деградация');
		sub = _('Сервис работает с предупреждениями.');
	} else if (state === 'down') {
		title = _('Маршрутизация недоступна');
		sub = isNativeEngine(data)
			? _('Engine, nft или control plane не отвечают.')
			: _('sing-box, nft или Clash API не отвечают.');
	} else {
		title = _('Нет данных');
		sub = _('Не удалось получить статус с роутера.');
	}
	var main = E('div', { 'class': 'hf-ent-hero__main' }, [
		E('p', { 'class': 'hf-ent-hero__title' }, title),
		E('p', { 'class': 'hf-ent-hero__sub' }, sub)
	]);
	var activeDisp = activeOutboundDisplay(data, opts.section);
	if (activeDisp) {
		main.appendChild(E('div', { 'class': 'hf-ent-hero__active' }, [
			_('Активный канал') + ': ',
			E('span', { 'class': 'hf-mon-tag' }, activeDisp)
		]));
	}
	var errs = visibleErrors(data);
	if (errs.length) {
		main.appendChild(E('div', { 'class': 'hf-ent-hero__sub', 'style': 'margin-top:8px;color:#c0392b;' },
			errs.join(' · ')));
	}
	if (opts.primaryError) {
		main.appendChild(E('div', { 'class': 'hf-ent-hero__sub', 'style': 'margin-top:6px;color:#c0392b;' },
			_('Primary') + ': ' + opts.primaryError));
	}
	var links = E('div', { 'class': 'hf-ent-hero__links' }, [
		E('a', { 'href': L.url('admin/services/hybrid-failover/routing') }, _('Маршрутизация')),
		E('a', { 'href': L.url('admin/services/hybrid-failover/diagnostics') }, _('Диагностика')),
		E('a', { 'href': L.url('admin/services/hybrid-failover/clients') }, _('Клиенты'))
	]);
	return E('div', { 'class': heroCls }, [main, links]);
}

function buildTabBar(tabs, activeId, onSelect) {
	var bar = E('div', { 'class': 'hf-ent-tabs', 'role': 'tablist' });
	for (var i = 0; i < tabs.length; i++) {
		(function(tab) {
			bar.appendChild(E('button', {
				'type': 'button',
				'class': 'hf-ent-tab' + (tab.id === activeId ? ' hf-ent-tab--active' : ''),
				'role': 'tab',
				'aria-selected': tab.id === activeId ? 'true' : 'false',
				'click': function(ev) {
					ev.preventDefault();
					onSelect(tab.id);
				}
			}, tab.label));
		})(tabs[i]);
	}
	return bar;
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
		text = isNativeEngine(data)
			? _('Engine, nft или control API недоступны.')
			: _('sing-box, nft или Clash API недоступны.');
	} else {
		cls = 'hf-mon-banner hf-mon-banner--sticky';
		title = _('Нет данных');
		text = _('Не удалось получить статус с роутера.');
	}
	var children = [E('strong', {}, title + ': '), text];
	var activeDisp = activeOutboundDisplay(data, opts.section);
	if (activeDisp) {
		children.push(E('span', { 'class': 'hf-mon-tag', 'style': 'display:block;margin-top:6px;' },
			_('Активный канал') + ': ' + activeDisp));
	}
	if (visibleErrors(data).length)
		children.push(E('div', { 'style': 'margin-top:8px;color:#c0392b;' }, visibleErrors(data).join(' · ')));
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

function buildMetricCards(data, section) {
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
	else {
		var ctrlPol = controllerForSection(data, section);
		if (ctrlPol && ctrlPol.policy)
			policyVal = ctrlPol.policy;
	}
	var engineLabel = isNativeEngine(data) ? 'Engine' : 'sing-box';
	var controlLabel = isNativeEngine(data) ? _('Control') : 'Clash API';
	var engineRunning = proxyRunning(data);
	var controlRunning = controlOk(data);
	var activeTag = activeOutboundTag(data, section);
	var activeState = activeTag ? 'ok' : 'neutral';
	var ctrl = controllerForSection(data, section);
	if (ctrl && ctrl.mode === 'backup' && !ctrl.primary_ok)
		activeState = 'warn';
	return E('div', { 'class': 'hf-mon-grid' }, [
		card(engineLabel, engineRunning ? _('работает') : _('остановлен'),
			engineRunning ? 'ok' : 'bad'),
		card('nft / tproxy', data && data.nft_ok ? 'OK' : _('ошибка'),
			data && data.nft_ok ? 'ok' : 'bad'),
		card(controlLabel, controlRunning ? 'OK' : _('недоступен'),
			controlRunning ? 'ok' : 'bad'),
		card('fakeip DNS', fakeipVal, fakeipState),
		card(_('Политика'), policyVal, 'info'),
		card(_('Активный outbound'), activeOutboundDisplay(data, section) || activeTag || '-', activeState),
		card(_('Резервы'), (function() {
			var ch = data && data.channels;
			if (!ch || !ch.length)
				return '-';
			var sum = channelsReserveSummary(ch, data, false);
			return sum || '-';
		})(), (function() {
			var ch = data && data.channels;
			if (!ch || !ch.length)
				return 'neutral';
			var ctrl = controllerForSection(data, section);
			var alive = 0, total = 0;
			for (var i = 0; i < ch.length; i++) {
				if (channelKind(ch[i]) !== 'reserve')
					continue;
				total++;
				if (channelAliveState(ch[i], false, ctrl) === 'up')
					alive++;
			}
			if (!total)
				return 'neutral';
			if (alive === total)
				return 'ok';
			if (alive === 0)
				return 'bad';
			return 'warn';
		})())
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
		[_('Selector'), fo && fo.selector_now ? E('span', { 'class': 'hf-mon-tag' }, fo.selector_now) :
			(ctrl && ctrl.active ? E('span', { 'class': 'hf-mon-tag' }, ctrl.active) : '-')],
		[_('URLTest'), (function() {
			var val = fo && fo.urltest_now;
			if (!val && ctrl && ctrl.urltest_member)
				val = ctrl.urltest_member;
			if (!val && ctrl && ctrl.active) {
				if (ctrl.active.indexOf('-urltest-out') !== -1)
					val = ctrl.active;
				else if (ctrl.active.indexOf('-awg-out') === -1 && ctrl.active !== ((ctrl.section || '') + '-out'))
					val = ctrl.active;
			}
			return val ? E('span', { 'class': 'hf-mon-tag' }, val) : '-';
		})()]
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

function buildChannelsTable(channels, probed, serverDelayData, nativeEngine, data) {
	if (!channels || !channels.length)
		return E('p', { 'class': 'hf-mon-empty' },
			_('Каналы не найдены. Включите VPN+failover в секции маршрутизации.'));

	var ctrl = controllerForSection(data, data && data.failover && data.failover.section);
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
		var role = channelRoleLabel(ch, ctrl);
		var statusCell = channelStatusBadge(ch, probed, ctrl);
		if (ch.detail && channelAliveState(ch, probed, ctrl) !== 'up') {
			statusCell = E('div', {}, [
				statusCell,
				E('div', { 'style': 'font-size:11px;opacity:.75;margin-top:2px;' }, ch.detail)
			]);
		}
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
				nativeEngine
					? _('Данные из engine. Нажмите «Live probe» для актуальной проверки.')
					: _('Данные из кэша Clash. Нажмите «Live probe» для актуальной проверки.'))
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
	if (d.engine_running != null)
		items.push({ ok: !!d.engine_running, text: 'engine: ' + (d.engine_running ? 'running' : 'stopped') });
	else if (d.singbox_running != null)
		items.push({ ok: !!d.singbox_running, text: 'engine: ' + (d.singbox_running ? 'running' : 'stopped') });
	if (d.nft_ok != null)
		items.push({ ok: !!d.nft_ok, text: 'nft: ' + (d.nft_ok ? 'OK' : 'FAIL') });
	if (d.fakeip_ok != null)
		items.push({ ok: !!d.fakeip_ok, text: 'fakeip: ' + (d.fakeip_ok ? 'OK' : 'FAIL') });
	if (!isNativeEngine(d) && d.clash_ok != null)
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
	proxyRunning: proxyRunning,
	controlOk: controlOk,
	isNativeEngine: isNativeEngine,
	controllerForSection: controllerForSection,
	activeOutboundTag: activeOutboundTag,
	activeOutboundDisplay: activeOutboundDisplay,
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
	buildStatusPill: buildStatusPill,
	buildEnterpriseMeta: buildEnterpriseMeta,
	buildStatusHero: buildStatusHero,
	buildTabBar: buildTabBar,
	buildSummaryBanner: buildSummaryBanner,
	buildMetricCards: buildMetricCards,
	buildChannelsOverview: buildChannelsOverview,
	buildFailoverPanels: buildFailoverPanels,
	buildChannelsTable: buildChannelsTable,
	channelsReserveSummary: channelsReserveSummary,
	channelAliveState: channelAliveState,
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
