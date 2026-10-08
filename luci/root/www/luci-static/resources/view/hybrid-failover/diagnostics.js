'use strict';
'require view';
'require rpc';
'require ui';
'require hybrid-failover.hf-ui as hfui';

function rpcCall(method, params) {
	var decl = { object: 'hybrid-failover', method: method };
	if (params)
		decl.params = params;
	return rpc.declare(decl);
}

// health/global-check can exceed LuCI's default 20s XHR timeout.
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

var callValidate = rpcCall('validate');
var callCheckNft = rpcCall('check_nft');
var callCheckFakeip = rpcCall('check_fakeip');
var callGlobalCheck = rpcCall('global_check');
var callBackupUCI = rpcCall('backup_uci');
var callBackupDownload = rpcCall('backup_download');
var callRestoreUCI = rpcCall('restore_uci', [ 'path' ]);

var DEFAULT_BACKUP = '/tmp/hybrid-failover-uci-backup.tar.gz';

function callGlobalCheckLong() {
	return withRpcTimeout(60, function() { return callGlobalCheck(); });
}

function formatResult(res) {
	if (!res)
		return _('Нет данных');
	if (res.data != null)
		return typeof res.data === 'string' ? res.data : JSON.stringify(res.data, null, 2);
	if (res.output)
		return res.output;
	return JSON.stringify(res, null, 2);
}

function resultOk(res) {
	return !!res && res.ok !== false && !(res.data && res.data.ok === false);
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	setResult: function(title, res, fallbackText) {
		var ok = resultOk(res);
		var items = hfui.parseChecklist(res);
		if (!items.length)
			items = [ { ok: ok, text: fallbackText || (ok ? _('Готово') : (res && (res.error || res.output)) || _('Ошибка')) } ];
		this._resultTitle.textContent = title;
		this._resultState.className = 'hf-pill ' + (ok ? 'hf-pill--ok' : 'hf-pill--bad');
		this._resultState.textContent = ok ? _('в порядке') : _('есть проблемы');
		this._resultState.style.display = '';
		hfui.emptyNode(this._checklistEl);
		this._checklistEl.appendChild(hfui.renderChecklist(items));
		this._rawEl.textContent = formatResult(res);
		this._rawWrap.style.display = '';
		this._resultPanel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
	},

	run: function(fn, title) {
		var self = this;
		this._resultTitle.textContent = title;
		this._resultState.className = 'hf-pill';
		this._resultState.textContent = _('выполняется…');
		this._resultState.style.display = '';
		return fn().then(function(res) {
			self.setResult(title, res);
			return res;
		}).catch(function(err) {
			self.setResult(title, { ok: false, error: String(err.message || err) }, String(err.message || err));
		});
	},

	checkButton: function(label, hint, fn, title) {
		var self = this;
		return E('div', { 'class': 'hf-switch-row' }, [
			E('div', {}, [
				E('label', {}, label),
				E('div', { 'class': 'hf-field__hint' }, hint)
			]),
			E('button', {
				'class': 'btn cbi-button cbi-button-action',
				'click': ui.createHandlerFn(this, function() { return self.run(fn, title || label); })
			}, _('Проверить'))
		]);
	},

	downloadBackup: function() {
		var self = this;
		return callBackupDownload().then(function(res) {
			var d = res.data || res;
			if (d && d.data && d.filename) {
				var raw = atob(d.data);
				var arr = new Uint8Array(raw.length);
				for (var i = 0; i < raw.length; i++)
					arr[i] = raw.charCodeAt(i);
				var blob = new Blob([arr], { type: 'application/gzip' });
				var a = document.createElement('a');
				a.href = URL.createObjectURL(blob);
				a.download = d.filename;
				a.click();
				self.setResult(_('Резервная копия'), res, _('Файл %s скачан').format(d.filename));
			} else {
				self.setResult(_('Резервная копия'), { ok: false, data: d }, _('Роутер не отдал файл'));
			}
		});
	},

	render: function() {
		var self = this;

		this._resultTitle = E('h3', { 'class': 'hf-panel__title' }, _('Результат'));
		this._resultState = E('span', { 'class': 'hf-pill', 'style': 'display:none;' }, '');
		this._checklistEl = E('div', {}, [
			E('div', { 'class': 'hf-checks__empty' }, _('Запустите проверку, и здесь появится результат.'))
		]);
		this._rawEl = E('pre', { 'class': 'hf-result hf-result--info', 'style': 'max-height:320px;' }, '');
		this._rawWrap = E('details', { 'class': 'hf-details', 'style': 'display:none;' }, [
			E('summary', {}, _('Ответ целиком')),
			this._rawEl
		]);
		this._resultPanel = E('div', { 'class': 'hf-panel hf-panel--wide' }, [
			E('div', { 'class': 'hf-panel__head' }, [ this._resultTitle, this._resultState ]),
			this._checklistEl,
			this._rawWrap
		]);

		var pathInput = E('input', { 'class': 'cbi-input-text', 'type': 'text', 'value': DEFAULT_BACKUP });

		var header = hfui.pageHeader({
			title: _('Диагностика'),
			pills: [ hfui.pill(_('Проверки ничего не меняют в настройках'), 'plain') ],
			actions: [
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, function() { return self.run(callGlobalCheckLong, _('Полная проверка')); })
				}, _('Полная проверка'))
			],
			hint: _('Полная проверка смотрит движок, перехват трафика, DNS и конфигурацию сразу. Занимает до минуты.')
		});

		var root = E('div', { 'class': 'hf-page hf-mon' }, [
			header,
			E('div', { 'class': 'hf-panels' }, [
				hfui.panel(_('Отдельные проверки'), _('Если полная проверка нашла проблему, здесь можно перепроверить одну часть.'), [
					this.checkButton(_('Конфигурация'), _('Собирает конфигурацию из настроек и проверяет, что движок её примет.'), callValidate),
					this.checkButton(_('Перехват трафика'), _('Проверяет, что правила nftables, которые заворачивают трафик в движок, на месте.'), callCheckNft),
					this.checkButton(_('DNS (fake-IP)'), _('Проверяет, что домены из списков получают служебные адреса и пойдут в туннель.'), callCheckFakeip)
				]),
				hfui.panel(_('Резервная копия настроек'), _('Копия всех настроек Hybrid Failover. Пригодится перед экспериментами или переносом на другой роутер.'), [
					E('div', { 'class': 'hf-panel__actions', 'style': 'margin-top:0;' }, [
						E('button', {
							'class': 'btn cbi-button cbi-button-action',
							'click': ui.createHandlerFn(this, function() { return self.downloadBackup(); })
						}, _('Скачать копию')),
						E('button', {
							'class': 'btn cbi-button cbi-button-neutral',
							'click': ui.createHandlerFn(this, function() {
								return callBackupUCI().then(function(res) {
									self.setResult(_('Резервная копия'), res, resultOk(res) ? _('Копия сохранена на роутере') : _('Не удалось сохранить копию'));
								});
							})
						}, _('Сохранить на роутере'))
					]),
					E('details', { 'class': 'hf-details' }, [
						E('summary', {}, _('Восстановить из копии')),
						E('div', { 'class': 'hf-field' }, [
							E('label', {}, _('Файл копии на роутере')),
							pathInput,
							E('div', { 'class': 'hf-field__hint' }, _('Текущие настройки будут заменены настройками из файла.'))
						]),
						E('button', {
							'class': 'btn cbi-button cbi-button-negative',
							'click': ui.createHandlerFn(this, function() {
								var path = pathInput.value.trim() || DEFAULT_BACKUP;
								if (!confirm(_('Заменить текущие настройки настройками из %s?').format(path)))
									return Promise.resolve();
								return callRestoreUCI(path).then(function(res) {
									self.setResult(_('Восстановление'), res, resultOk(res) ? _('Настройки восстановлены') : _('Не удалось восстановить'));
								});
							})
						}, _('Восстановить'))
					])
				]),
				this._resultPanel
			])
		]);
		hfui.injectStyles(root);
		return root;
	}
});
