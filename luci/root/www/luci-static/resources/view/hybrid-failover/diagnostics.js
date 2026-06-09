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

var callValidate = rpcCall('validate');
var callCheckNft = rpcCall('check_nft');
var callCheckFakeip = rpcCall('check_fakeip');
var callGlobalCheck = rpcCall('global_check');
var callBackupUCI = rpcCall('backup_uci');
var callBackupDownload = rpcCall('backup_download');
var callRestoreUCI = rpcCall('restore_uci', [ 'path' ]);

function formatResult(res) {
	if (!res)
		return _('Нет данных');
	if (res.data != null)
		return JSON.stringify(res.data, null, 2);
	if (res.output)
		return res.output;
	return JSON.stringify(res, null, 2);
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	_checklistEl: null,
	_rawEl: null,
	_rawWrap: null,

	setResult: function(title, res) {
		var items = hfui.parseChecklist(res);
		if (this._checklistEl) {
			hfui.emptyNode(this._checklistEl);
			this._checklistEl.appendChild(hfui.renderChecklist(items));
		}
		if (this._rawEl)
			this._rawEl.textContent = title + '\n' + formatResult(res);
	},

	handleRpc: function(fn, title) {
		var self = this;
		return fn().then(function(res) {
			self.setResult(title, res);
			var ok = res && res.ok !== false && !(res.data && res.data.ok === false);
			ui.addNotification(null, E('strong', {}, title + (ok ? ' OK' : ' FAIL')), ok ? 'info' : 'danger');
			return res;
		}).catch(function(err) {
			self.setResult(title, { ok: false, error: String(err.message || err) });
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	render: function() {
		var self = this;
		this._checklistEl = E('div', {});
		this._rawEl = E('pre', {
			'style': 'white-space:pre-wrap;font-size:12px;max-height:320px;overflow:auto;margin:0;padding:12px;background:var(--cbi-section-background-color,rgba(127,127,127,.06));border-radius:8px;'
		}, '-');
		this._rawWrap = E('details', { 'style': 'margin-top:12px;' }, [
			E('summary', { 'style': 'cursor:pointer;font-weight:600;margin-bottom:8px;' }, _('Raw JSON')),
			this._rawEl
		]);

		var root = E('div', { 'class': 'cbi-section hf-mon' }, [
			E('h2', {}, _('Hybrid Failover: диагностика')),
			E('p', { 'class': 'hint' }, _('Проверки конфигурации и сетевого стека. Результаты — чеклист и raw JSON ниже.')),
			E('h3', {}, _('First-run')),
			E('p', { 'class': 'hint' }, _('1) hybrid-failover migrate  2) validate  3) apply  4) check-fakeip. Для VPN+резервы рекомендуется outage-only.')),
			E('div', { 'class': 'hf-mon-toolbar' }, [
				E('button', { 'class': 'btn cbi-button cbi-button-action', 'click': ui.createHandlerFn(self, function() { return self.handleRpc(callValidate, 'validate'); }) }, _('Validate')),
				E('button', { 'class': 'btn cbi-button cbi-button-action', 'click': ui.createHandlerFn(self, function() { return self.handleRpc(callCheckNft, 'nft'); }) }, _('check-nft')),
				E('button', { 'class': 'btn cbi-button cbi-button-action', 'click': ui.createHandlerFn(self, function() { return self.handleRpc(callCheckFakeip, 'fakeip'); }) }, _('check-fakeip')),
				E('button', { 'class': 'btn cbi-button cbi-button-save', 'click': ui.createHandlerFn(self, function() { return self.handleRpc(callGlobalCheck, 'global-check'); }) }, _('global-check'))
			]),
			E('h3', { 'style': 'margin-top:20px;' }, _('Результат')),
			E('div', { 'class': 'hf-mon-panel', 'style': 'margin-bottom:16px;' }, this._checklistEl),
			this._rawWrap,
			E('h3', { 'style': 'margin-top:20px;' }, _('Бэкап UCI')),
			E('div', { 'class': 'hf-mon-toolbar' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return callBackupUCI().then(function(res) {
							self.setResult(_('Create backup'), res);
							var ok = res && res.ok !== false;
							ui.addNotification(null, E('p', {}, ok ? _('Бэкап создан на роутере') : _('Ошибка создания бэкапа')), ok ? 'info' : 'danger');
						});
					})
				}, _('Создать backup на роутере')),
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(self, function() {
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
							}
							self.setResult(_('Download backup'), res);
						});
					})
				}, _('Скачать бэкап')),
				E('button', {
					'class': 'btn cbi-button cbi-button-negative',
					'click': ui.createHandlerFn(self, function() {
						var pathEl = document.getElementById('hf-backup-path');
						var path = pathEl ? pathEl.value.trim() : '/tmp/hybrid-failover-uci-backup.tar.gz';
						if (!confirm(_('Восстановить UCI из ') + path + '?'))
							return Promise.resolve();
						return callRestoreUCI(path).then(function(res) {
							self.setResult(_('Restore'), res);
						});
					})
				}, _('Восстановить'))
			]),
			E('input', { 'id': 'hf-backup-path', 'class': 'cbi-input-text', 'style': 'width:100%;max-width:480px;margin-top:8px;', 'value': '/tmp/hybrid-failover-uci-backup.tar.gz' })
		]);
		hfui.injectStyles(root);
		return root;
	}
});
