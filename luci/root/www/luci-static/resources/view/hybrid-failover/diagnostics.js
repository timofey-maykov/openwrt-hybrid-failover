'use strict';
'require view';
'require rpc';
'require ui';

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
var callRestoreUCI = rpcCall('restore_uci', [ 'path' ]);

function showResult(title, res) {
	var text = (res && res.data) ? JSON.stringify(res.data, null, 2) :
		(res && res.output) ? res.output :
		(res && res.message) ? res.message : _('Готово');
	var ok = res && res.ok !== false && !(res.data && res.data.ok === false);
	ui.addNotification(null, E('div', [
		E('strong', {}, title),
		E('pre', { 'style': 'white-space:pre-wrap;margin:8px 0 0;font-size:12px;' }, text)
	]), ok ? 'info' : 'danger');
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	handleRpc: function(fn, title) {
		return fn().then(function(res) {
			showResult(title, res);
			return res;
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	render: function() {
		var self = this;

		return E('div', { 'class': 'cbi-section' }, [
			E('h2', {}, _('Hybrid Failover: диагностика')),
			E('p', { 'class': 'hint' },
				_('Проверки конфигурации и сетевого стека без SSH. Результаты: во всплывающих уведомлениях.')),
			E('div', { 'style': 'display:flex;flex-wrap:wrap;gap:8px;margin:12px 0;' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return self.handleRpc(callValidate, _('Validate'));
					})
				}, _('Validate (pending)')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return self.handleRpc(callCheckNft, 'nft');
					})
				}, _('check-nft')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						return self.handleRpc(callCheckFakeip, 'fakeip');
					})
				}, _('check-fakeip')),
				E('button', {
					'class': 'btn cbi-button cbi-button-save',
					'click': ui.createHandlerFn(self, function() {
						return self.handleRpc(callGlobalCheck, _('global-check'));
					})
				}, _('global-check'))
			]),
			E('h3', { 'style': 'margin-top:20px;' }, _('Бэкап и восстановление UCI')),
			E('p', { 'class': 'hint' },
				_('Бэкап создаёт /tmp/hybrid-failover-uci-backup.tar.gz на роутере. Скачайте файл по SCP перед восстановлением.')),
			E('div', { 'style': 'display:flex;flex-wrap:wrap;gap:8px;margin-bottom:12px;' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-neutral',
					'click': ui.createHandlerFn(self, function() {
						return callBackupUCI().then(function(res) {
							var path = (res.data && res.data.path) || '/tmp/hybrid-failover-uci-backup.tar.gz';
							var el = document.getElementById('hf-backup-path');
							if (el)
								el.value = path;
							showResult(_('Бэкап UCI'), res);
						});
					})
				}, _('Создать бэкап')),
				E('button', {
					'class': 'btn cbi-button cbi-button-negative',
					'click': ui.createHandlerFn(self, function() {
						var pathEl = document.getElementById('hf-backup-path');
						var path = pathEl ? pathEl.value.trim() : '/tmp/hybrid-failover-uci-backup.tar.gz';
						if (!confirm(_('Восстановить UCI из ') + path + '? Текущий конфиг будет перезаписан.'))
							return Promise.resolve();
						return callRestoreUCI(path).then(function(res) {
							showResult(_('Восстановление'), res);
						}).catch(function(err) {
							ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
						});
					})
				}, _('Восстановить из архива'))
			]),
			E('label', {}, _('Путь к архиву на роутере')),
			E('input', {
				'id': 'hf-backup-path',
				'class': 'cbi-input-text',
				'style': 'width:100%;max-width:480px;',
				'value': '/tmp/hybrid-failover-uci-backup.tar.gz'
			})
		]);
	}
});
