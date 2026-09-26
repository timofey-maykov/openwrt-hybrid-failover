'use strict';
'require view';
'require rpc';
'require ui';
'require poll';

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

var stepNames = {
	start: _('запуск'),
	release: _('поиск релиза'),
	manifest: _('чтение списка пакетов'),
	download: _('скачивание'),
	install: _('установка пакетов'),
	restart: _('перезапуск служб'),
	done: _('готово')
};

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	_status: null,
	_polling: false,

	load: function() {
		return callStatus().then(dataOf).catch(function() { return {}; });
	},

	renderInfo: function() {
		var s = this._status || {};
		var chk = s.check || null;
		var st = s.state || { state: 'idle' };
		var rows = [];

		rows.push(E('tr', { 'class': 'tr' }, [
			E('td', { 'class': 'td left', 'width': '33%' }, _('Установлена')),
			E('td', { 'class': 'td left' }, E('strong', {}, s.installed || '?'))
		]));

		var latest;
		if (!chk)
			latest = E('em', {}, _('ещё не проверялось'));
		else if (chk.error)
			latest = E('span', { 'style': 'color:#c00' }, _('ошибка проверки: ') + chk.error);
		else
			latest = E('span', {}, [
				E('strong', {}, chk.latest || '?'),
				chk.published_at ? ' (' + fmtDate(chk.published_at) + ')' : '',
				chk.available ? E('span', { 'class': 'label notice', 'style': 'margin-left:8px' }, _('есть обновление')) :
					E('span', { 'style': 'margin-left:8px;color:#080' }, _('установлена последняя'))
			]);
		rows.push(E('tr', { 'class': 'tr' }, [
			E('td', { 'class': 'td left' }, _('Последняя на GitHub')),
			E('td', { 'class': 'td left' }, latest)
		]));
		if (chk && chk.checked_at)
			rows.push(E('tr', { 'class': 'tr' }, [
				E('td', { 'class': 'td left' }, _('Проверено')),
				E('td', { 'class': 'td left' }, new Date(chk.checked_at * 1000).toLocaleString())
			]));

		var children = [ E('table', { 'class': 'table' }, rows) ];

		if (chk && chk.available && chk.notes)
			children.push(E('details', { 'style': 'margin-top:10px', 'open': 'open' }, [
				E('summary', { 'style': 'cursor:pointer;font-weight:600' }, _('Что нового в ') + chk.latest),
				E('pre', { 'style': 'white-space:pre-wrap;font-size:12px;max-height:300px;overflow:auto' }, chk.notes)
			]));

		if (st.state && st.state !== 'idle') {
			var color = st.state === 'failed' ? '#c00' : (st.state === 'done' ? '#080' : 'inherit');
			var head = st.state === 'running' ? _('Идёт обновление: ') + (stepNames[st.step] || st.step || '') :
				st.state === 'done' ? _('Обновление до %s установлено').format(st.to || '') :
				_('Обновление не удалось');
			children.push(E('div', { 'style': 'margin-top:12px' }, [
				E('p', { 'style': 'font-weight:600;color:' + color }, [
					st.state === 'running' ? E('span', { 'class': 'spinning' }, ' ') : '',
					head
				]),
				st.message ? E('p', {}, st.message) : '',
				(st.log && st.log.length) ? E('details', {}, [
					E('summary', { 'style': 'cursor:pointer' }, _('Журнал')),
					E('pre', { 'style': 'white-space:pre-wrap;font-size:12px;max-height:240px;overflow:auto' }, st.log.join('\n'))
				]) : ''
			]));
		}
		return E('div', {}, children);
	},

	refresh: function() {
		var info = document.getElementById('hf-update-info');
		if (info)
			info.replaceChildren(this.renderInfo());
		var s = this._status || {};
		var running = s.state && s.state.state === 'running';
		var available = s.check && s.check.available;
		var btnApply = document.getElementById('hf-update-apply');
		var btnCheck = document.getElementById('hf-update-check');
		if (btnApply) {
			btnApply.disabled = !available || running;
			btnApply.textContent = available ? _('Обновить до %s').format(s.check.latest) : _('Обновить');
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

		var node = E('div', { 'class': 'cbi-section hf-mon' }, [
			E('h2', {}, _('Hybrid Failover: обновление')),
			E('p', { 'class': 'hint' }, _('Проверка новых версий на GitHub и установка из релиза. Ставятся только пакеты для этого роутера, каждый файл сверяется с контрольной суммой из релиза. Настройки сохраняются.')),
			E('div', { 'id': 'hf-update-info' }, this.renderInfo()),
			E('div', { 'class': 'hf-mon-toolbar', 'style': 'margin-top:12px' }, [
				E('button', {
					'id': 'hf-update-check',
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, 'handleCheck')
				}, _('Проверить обновления')),
				' ',
				E('button', {
					'id': 'hf-update-apply',
					'class': 'btn cbi-button cbi-button-save',
					'click': ui.createHandlerFn(self, 'handleApply')
				}, _('Обновить'))
			])
		]);

		window.setTimeout(function() {
			self.refresh();
			if (self._status.state && self._status.state.state === 'running')
				self.startPolling();
		}, 0);
		return node;
	}
});
