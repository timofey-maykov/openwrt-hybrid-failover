'use strict';
'require view';
'require rpc';
'require ui';
'require poll';
'require hybrid-failover.hf-ui as hfui';

// Update Hybrid Failover from its GitHub releases. The check and the install
// run in the core binary (hybrid-failover update ...); the install runs
// detached because it restarts rpcd, so this page follows it by polling
// update_status and simply keeps trying while rpcd is away.

var callStatus = rpc.declare({ object: 'hybrid-failover', method: 'update_status' });
var callCheck = rpc.declare({ object: 'hybrid-failover', method: 'update_check' });
var callApply = rpc.declare({ object: 'hybrid-failover', method: 'update_apply' });

function withRpcTimeout(seconds, fn) {
	var prev = L.env.rpctimeout;
	L.env.rpctimeout = seconds;
	return Promise.resolve().then(fn).finally(function() {
		if (prev == null)
			delete L.env.rpctimeout;
		else
			L.env.rpctimeout = prev;
	});
}

function dataOf(res) {
	if (res && res.data != null)
		return res.data;
	return res || {};
}

function fmtDate(s) {
	if (!s)
		return '';
	var d = new Date(s);
	return isNaN(d.getTime()) ? s : d.toLocaleString();
}

var STEPS = [ 'start', 'release', 'manifest', 'download', 'install', 'restart', 'done' ];

var stepNames = {
	start: _('запуск'),
	release: _('поиск релиза'),
	manifest: _('чтение списка пакетов'),
	download: _('скачивание'),
	install: _('установка пакетов'),
	restart: _('перезапуск служб'),
	done: _('готово')
};

function cmpVersion(a, b) {
	var pa = String(a || '').replace(/^v/, '').split(/[.-]/);
	var pb = String(b || '').replace(/^v/, '').split(/[.-]/);
	for (var i = 0; i < Math.max(pa.length, pb.length); i++) {
		var x = parseInt(pa[i] || '0', 10), y = parseInt(pb[i] || '0', 10);
		if (isNaN(x) || isNaN(y))
			return 0;
		if (x !== y)
			return x > y ? 1 : -1;
	}
	return 0;
}

var UPDATE_CSS = [
	'.hf-steps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }',
	'.hf-steps li { display: flex; align-items: center; gap: 10px; font-size: 13px; color: var(--hf-muted); padding: 4px 0; }',
	'.hf-steps li::before { content: ""; width: 10px; height: 10px; border-radius: 50%; border: 2px solid var(--hf-line); flex: none; }',
	'.hf-steps li.hf-steps--done { color: inherit; } .hf-steps li.hf-steps--done::before { background: var(--hf-ok); border-color: var(--hf-ok); }',
	'.hf-steps li.hf-steps--now { color: inherit; font-weight: 700; } .hf-steps li.hf-steps--now::before { border-color: var(--hf-acc); background: var(--hf-acc-soft); }',
	'.hf-steps li.hf-steps--fail { color: var(--hf-bad); font-weight: 700; } .hf-steps li.hf-steps--fail::before { background: var(--hf-bad); border-color: var(--hf-bad); }',
	'.hf-notes { white-space: pre-wrap; font-size: 13px; line-height: 1.5; max-height: 360px; overflow: auto; margin: 0; }'
].join('\n');

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	_status: null,
	_polling: false,

	load: function() {
		return callStatus().then(dataOf).catch(function() { return {}; });
	},

	state: function() {
		var s = this._status || {};
		var chk = s.check || null;
		var newer = chk && chk.latest && s.installed && cmpVersion(chk.latest, s.installed) > 0;
		var ahead = chk && chk.latest && s.installed && cmpVersion(s.installed, chk.latest) > 0;
		return {
			s: s,
			chk: chk,
			st: s.state || { state: 'idle' },
			available: !!(chk && chk.available && newer !== false),
			ahead: !!ahead
		};
	},

	renderPills: function() {
		var x = this.state();
		var pills = [ hfui.pill(_('Установлена ') + (x.s.installed || '?'), 'plain') ];
		if (x.st.state === 'running')
			pills.push(hfui.pill(_('Идёт обновление'), 'warn'));
		else if (!x.chk)
			pills.push(hfui.pill(_('Обновления ещё не проверялись'), ''));
		else if (x.chk.error)
			pills.push(hfui.pill(_('Не удалось проверить'), 'bad'));
		else if (x.available)
			pills.push(hfui.pill(_('Есть обновление до ') + x.chk.latest, 'warn'));
		else
			pills.push(hfui.pill(_('Последняя версия'), 'ok'));
		return pills;
	},

	renderInfo: function() {
		var x = this.state();
		var chk = x.chk, st = x.st;
		var panels = [];

		var latest;
		if (!chk)
			latest = _('ещё не проверялось');
		else if (chk.error)
			latest = E('span', { 'style': 'color:var(--hf-bad);' }, chk.error);
		else
			latest = (chk.latest || '?') + (chk.published_at ? ', ' + fmtDate(chk.published_at) : '');

		var note = '';
		if (x.ahead)
			note = E('p', { 'class': 'hf-panel__sub', 'style': 'margin-top:12px;' },
				_('Установлена версия новее, чем при последней проверке. Нажмите «Проверить обновления», чтобы узнать свежие данные.'));

		panels.push(hfui.panel(_('Версии'), _('Пакеты берутся из релизов на GitHub. Ставятся только пакеты для этого роутера, каждый файл сверяется с контрольной суммой. Настройки сохраняются.'), [
			hfui.kvList([
				[ _('Установлена'), x.s.installed || '?' ],
				[ _('Последняя на GitHub'), latest ],
				chk && chk.checked_at ? [ _('Проверено'), new Date(chk.checked_at * 1000).toLocaleString() ] : null
			]),
			note
		]));

		if (st.state && st.state !== 'idle') {
			var cur = STEPS.indexOf(st.step);
			if (st.state === 'done')
				cur = STEPS.length;
			var steps = E('ul', { 'class': 'hf-steps' }, STEPS.filter(function(k) { return k !== 'done'; }).map(function(k, i) {
				var cls = '';
				if (i < cur)
					cls = 'hf-steps--done';
				else if (i === cur)
					cls = st.state === 'failed' ? 'hf-steps--fail' : 'hf-steps--now';
				return E('li', { 'class': cls }, stepNames[k]);
			}));
			var title = st.state === 'running' ? _('Идёт обновление') :
				st.state === 'done' ? _('Обновление до %s установлено').format(st.to || '') :
				_('Обновление не удалось');
			panels.push(hfui.panel(title, st.message || '', [
				steps,
				(st.log && st.log.length) ? E('details', { 'class': 'hf-details' }, [
					E('summary', {}, _('Журнал')),
					E('pre', { 'class': 'hf-result hf-result--info' }, st.log.join('\n'))
				]) : ''
			]));
		}

		if (x.available && chk.notes)
			panels.push(hfui.panel(_('Что нового в ') + chk.latest, '', [
				E('pre', { 'class': 'hf-notes' }, chk.notes)
			], { wide: true }));

		return E('div', { 'class': 'hf-panels' }, panels);
	},

	refresh: function() {
		var info = document.getElementById('hf-update-info');
		if (info)
			info.replaceChildren(this.renderInfo());
		if (this._header)
			this._header.setPills(this.renderPills());
		var x = this.state();
		var running = x.st.state === 'running';
		var btnApply = document.getElementById('hf-update-apply');
		var btnCheck = document.getElementById('hf-update-check');
		if (btnApply) {
			btnApply.style.display = x.available ? '' : 'none';
			btnApply.disabled = running;
			btnApply.textContent = x.available ? _('Обновить до %s').format(x.chk.latest) : _('Обновить');
		}
		if (btnCheck)
			btnCheck.disabled = running;
	},

	startPolling: function() {
		var self = this;
		if (this._polling)
			return;
		this._polling = true;
		var tick = function() {
			return callStatus().then(dataOf).then(function(s) {
				self._status = s;
				self.refresh();
				var state = s.state && s.state.state;
				if (state !== 'running') {
					poll.remove(tick);
					self._polling = false;
					if (state === 'done') {
						ui.addNotification(null, E('p', {}, _('Hybrid Failover обновлён до %s. Страница перезагрузится.').format(s.state.to || '')), 'info');
						window.setTimeout(function() { location.reload(); }, 3000);
					}
				}
			}).catch(function() {
				// rpcd restarts during the install; keep waiting.
			});
		};
		poll.add(tick, 2);
	},

	handleCheck: function() {
		var self = this;
		return withRpcTimeout(100, function() { return callCheck(); }).then(function() {
			return callStatus();
		}).then(dataOf).then(function(s) {
			self._status = s;
			self.refresh();
			var chk = s.check || {};
			if (chk.error)
				ui.addNotification(null, E('p', {}, _('Не удалось проверить: ') + chk.error), 'danger');
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	handleApply: function() {
		var self = this;
		var s = this._status || {};
		var to = s.check ? s.check.latest : '';
		if (!confirm(_('Установить Hybrid Failover %s? Службы будут перезапущены, связь с роутером может пропасть на несколько секунд.').format(to)))
			return Promise.resolve();
		return callApply().then(dataOf).then(function(r) {
			if (r && r.started === false) {
				ui.addNotification(null, E('p', {}, _('Обновление не запущено: ') + (r.error || '')), 'danger');
				return;
			}
			self._status.state = { state: 'running', step: 'start' };
			self.refresh();
			self.startPolling();
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	render: function(status) {
		var self = this;
		this._status = status || {};
		this._header = hfui.pageHeader({
			title: _('Обновление Hybrid Failover'),
			pills: this.renderPills(),
			actions: [
				E('button', {
					'id': 'hf-update-check',
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(this, 'handleCheck')
				}, _('Проверить обновления')),
				E('button', {
					'id': 'hf-update-apply',
					'class': 'btn cbi-button cbi-button-apply',
					'style': 'display:none;',
					'click': ui.createHandlerFn(this, 'handleApply')
				}, _('Обновить'))
			]
		});
		var node = E('div', { 'class': 'hf-page hf-mon' }, [
			E('style', { 'type': 'text/css' }, UPDATE_CSS),
			this._header,
			E('div', { 'id': 'hf-update-info' }, this.renderInfo())
		]);
		hfui.injectStyles(node);
		window.setTimeout(function() {
			self.refresh();
			if (self._status.state && self._status.state.state === 'running')
				self.startPolling();
		}, 0);
		return node;
	}
});
