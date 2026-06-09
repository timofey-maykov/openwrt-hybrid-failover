'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';
'require hybrid-failover.hf-ui as hfui';

var callValidateConfig = rpc.declare({
	object: 'hybrid-failover',
	method: 'validate'
});

var callApplyConfig = rpc.declare({
	object: 'hybrid-failover',
	method: 'apply'
});

var callPendingRollback = rpc.declare({
	object: 'hybrid-failover',
	method: 'pending_rollback'
});

var callDecodeURI = rpc.declare({
	object: 'hybrid-failover',
	method: 'decode_uri',
	params: [ 'uri' ]
});

var callDuplicateSection = rpc.declare({
	object: 'hybrid-failover',
	method: 'duplicate_section',
	params: [ 'from', 'to' ]
});

var callListUpdate = rpc.declare({
	object: 'hybrid-failover',
	method: 'list_update'
});

var callSubscriptionRefresh = rpc.declare({
	object: 'hybrid-failover',
	method: 'subscription_refresh'
});

var _validateTimer = null;

function formatStepOutput(res) {
	if (!res || res.ok === false)
		return (res && (res.output || res.error)) ? String(res.output || res.error) : _('Ошибка');
	if (res.output)
		return String(res.output);
	if (res.data != null && typeof res.data === 'object' && !Array.isArray(res.data))
		return JSON.stringify(res.data, null, 2);
	if (res.data != null)
		return JSON.stringify(res.data, null, 2);
	return _('OK');
}

function scheduleValidate(self) {
	if (_validateTimer)
		clearTimeout(_validateTimer);
	_validateTimer = setTimeout(function() {
		if (!self || !self.map)
			return;
		self.map.save(false).then(function() {
			return callValidateConfig();
		}).then(function(res) {
			if (res && res.ok === false)
				notifyRpcResult(_('Проверка'), res);
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	}, 2000);
}

function swapListItem(pkg, section, opt, idx, dir) {
	return uci.get(pkg, section, opt).then(function(list) {
		if (!Array.isArray(list))
			list = list ? [list] : [];
		var j = idx + dir;
		if (j < 0 || j >= list.length)
			return Promise.resolve();
		var tmp = list[idx];
		list[idx] = list[j];
		list[j] = tmp;
		uci.set(pkg, section, opt, list);
		return uci.save();
	});
}

var COMMUNITY_LISTS = {
	russia_inside: _('Russia inside'),
	russia_outside: _('Russia outside'),
	ukraine_inside: _('Ukraine inside'),
	geoblock: _('Geoblock'),
	block: _('Block'),
	porn: _('Porn'),
	news: _('News'),
	anime: _('Anime'),
	youtube: _('YouTube'),
	hdrezka: _('HDRezka'),
	tiktok: _('TikTok'),
	google_ai: _('Google AI'),
	google_play: _('Google Play'),
	hodca: _('Hodca'),
	discord: _('Discord'),
	meta: _('Meta'),
	twitter: _('Twitter'),
	cloudflare: _('Cloudflare'),
	cloudfront: _('Cloudfront'),
	digitalocean: _('DigitalOcean'),
	hetzner: _('Hetzner'),
	ovh: _('OVH'),
	telegram: _('Telegram'),
	roblox: _('Roblox'),
	netflix: _('Netflix')
};

function notifyRpcResult(title, res) {
	hfui.notifyRpcResult(title, res);
}

return view.extend({
	load: function() {
		return uci.load('hybrid-failover');
	},

	handleRpc: function(fn, title) {
		var p = fn();
		if (!p || typeof p.then !== 'function')
			return Promise.resolve();
		return p.then(function(res) {
			notifyRpcResult(title, res);
		}).catch(function(err) {
			ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
		});
	},

	render: function() {
		var m, st, s, o, self = this;

		m = new form.Map('hybrid-failover', _('Hybrid Failover: маршрутизация'),
			_('Настройка VPN + failover, URLTest и подписок. Сохраните UCI, затем «Проверить» и «Применить».'));

		st = m.section(form.NamedSection, 'settings', 'settings', _('Глобальные настройки'));

		o = st.option(form.Flag, 'enabled', _('Включить Hybrid Failover'));
		o.default = '1';

		o = st.option(form.ListValue, 'dns_type', _('Тип DNS'));
		o.value('doh', _('DoH'));
		o.value('dot', _('DoT'));
		o.value('udp', _('UDP'));

		o = st.option(form.Value, 'dns_server', _('DNS-сервер'));
		o.placeholder = '1.1.1.1';

		o = st.option(form.Value, 'bootstrap_dns_server', _('Bootstrap DNS'));
		o.placeholder = '77.88.8.8';

		o = st.option(form.Value, 'cache_path', _('Путь cache sing-box'));
		o.placeholder = '/etc/sing-box/cache.db';

		o = st.option(form.Flag, 'disable_quic', _('Отключить QUIC в маршрутизации'));

		o = st.option(form.Value, 'dns_rewrite_ttl', _('DNS rewrite TTL (сек)'));
		o.placeholder = '60';

		o = st.option(form.Value, 'clash_api_listen', _('Clash API listen'));
		o.placeholder = '192.168.42.1:9090';

		o = st.option(form.Flag, 'enable_yacd', _('Включить Yacd (Clash UI)'));

		o = st.option(form.DummyValue, '_yacd_link', _('Открыть Clash UI'));
		o.depends('enable_yacd', '1');
		o.renderWidget = function() {
			function yacdWidget(listen) {
				listen = String(listen || '127.0.0.1:9090').trim();
				var url = listen.indexOf('://') >= 0 ? listen : ('http://' + listen);
				if (url.slice(-1) !== '/')
					url += '/';
				url += 'ui';
				return E('div', {}, [
					E('a', { 'href': url, 'target': '_blank', 'rel': 'noopener' }, url),
					E('p', { 'class': 'hint' },
						_('Не включайте WAN access без необходимости: API будет доступен извне.'))
				]);
			}
			var listen = uci.get('hybrid-failover', 'settings', 'clash_api_listen');
			if (listen != null && typeof listen.then === 'function')
				return listen.then(yacdWidget);
			return yacdWidget(listen);
		};

		o = st.option(form.Flag, 'enable_yacd_wan_access', _('Clash API на WAN (0.0.0.0)'));
		o.depends('enable_yacd', '1');

		o = st.option(form.Value, 'yacd_secret_key', _('Clash API secret (Bearer)'));
		o.depends('enable_yacd', '1');

		o = st.option(form.Value, 'output_network_interface', _('Исходящий сетевой интерфейс'));
		o.placeholder = _('пусто = авто');

		o = st.option(form.Value, 'main_section', _('Основная секция маршрутизации'));
		o.placeholder = 'glob';

		o = st.option(form.Value, 'update_interval', _('Интервал обновления списков'));
		o.placeholder = '1d';

		o = st.option(form.Flag, 'download_lists_via_proxy', _('Скачивать community-списки через proxy'));

		o = st.option(form.Value, 'download_lists_via_proxy_section', _('Секция proxy для загрузки списков'));
		o.placeholder = 'glob';
		o.depends('download_lists_via_proxy', '1');

		o = st.option(form.Value, 'webhook_url', _('Webhook URL (failover)'));
		o.placeholder = 'https://…';

		o = st.option(form.Flag, 'dont_touch_dhcp', _('Не изменять dnsmasq/DHCP'));

		o = st.option(form.DynamicList, 'routing_excluded_ips', _('Исключить IP из маршрутизации'));
		o.description = _('Глобально не направлять эти IP через sing-box (не путать с вкладкой «Клиенты»).');
		o.placeholder = '192.168.1.100';

		o = st.option(form.Value, 'failover_probe_interval', _('Интервал probe контроллера'));
		o.placeholder = '30s';
		o.description = _('Как часто фоновый controller проверяет primary VPN (не URLTest interval).');

		o = st.option(form.Value, 'history_max_lines', _('Макс. строк в журнале failover'));
		o.placeholder = '500';

		o = st.option(form.Value, 'delay_history_points', _('Точек delay-history на канал'));
		o.placeholder = '50';
		o.description = _('Размер буфера задержек в /var/run/hybrid-failover/delay-history.json для графиков в обзоре.');

		o = st.option(form.DynamicList, 'subscription_urls', _('Subscription URLs'));

		s = m.section(form.TypedSection, 'section', _('Секции маршрутизации'));
		s.anonymous = false;
		s.addremove = true;

		o = s.option(form.Flag, 'enabled', _('Включить секцию'));
		o.default = '1';

		o = s.option(form.ListValue, 'connection_type', _('Тип подключения'));
		o.value('vpn', _('VPN'));
		o.value('proxy', _('Proxy'));
		o.value('block', _('Block'));

		o = s.option(form.Value, 'interface', _('VPN-интерфейс'));
		o.placeholder = 'awg0';
		o.depends('connection_type', 'vpn');

		o = s.option(form.Flag, 'failover_vpn_enabled', _('VPN + резервные proxy'));
		o.depends('connection_type', 'vpn');

		o = s.option(form.ListValue, 'failover_policy', _('Политика failover'));
		o.value('outage-only', _('outage-only: только при падении VPN'));
		o.value('prefer-primary', _('prefer-primary: вернуться на VPN раньше'));
		o.value('fastest', _('fastest: urltest в sing-box (controller passive)'));
		o.default = 'outage-only';
		o.description = _('fastest: sing-box urltest выбирает канал; controller только наблюдает. Для VPN→backup при падении используйте outage-only.') +
			' ' + _('Подробнее на вкладке «Обзор».');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.DynamicList, 'failover_proxy_links', _('Резервные URI'));
		o.description = _('Порядок в списке = приоритет резервов. vpn:// и awg2:// поддерживаются.');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'failover_fail_threshold', _('Порог сбоев primary (fail)'));
		o.placeholder = '2';
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'failover_recover_threshold', _('Порог восстановления (recover)'));
		o.placeholder = '2';
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.DummyValue, '_uri_preview', _('Превью URI'));
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });
		o.renderWidget = function(section_id) {
			var wrap = E('div', { 'id': 'hf-uri-preview-' + section_id });
			var input = E('input', {
				'class': 'cbi-input-text',
				'style': 'width:100%;margin-bottom:6px;',
				'placeholder': 'vless://… или vpn://…'
			});
			var btn = E('button', {
				'class': 'btn cbi-button cbi-button-action',
				'click': ui.createHandlerFn(self, function() {
					var uri = input.value.trim();
					if (!uri) {
						out.textContent = _('Введите URI');
						return Promise.resolve();
					}
					return callDecodeURI(uri).then(function(res) {
						var d = res.data || res;
						var text = d.summary || d.error || JSON.stringify(d, null, 2);
						hfui.showModal(_('Превью URI'), [
							E('pre', { 'style': 'white-space:pre-wrap;font-size:12px;margin:0;max-height:280px;overflow:auto;' }, text)
						]);
					}).catch(function(err) {
						hfui.showModal(_('Превью URI'), [
							E('p', {}, String(err.message || err))
						]);
					});
				})
			}, _('Проверить URI'));
			wrap.appendChild(input);
			wrap.appendChild(btn);
			return wrap;
		};

		o = s.option(form.DummyValue, '_failover_reorder', _('Порядок резервов'));
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });
		o.renderWidget = function(section_id) {
			var idxInput = E('input', {
				'type': 'number',
				'min': '0',
				'class': 'cbi-input-text',
				'style': 'width:80px;',
				'value': '0'
			});
			return E('div', { 'style': 'display:flex;gap:8px;flex-wrap:wrap;align-items:center;' }, [
				E('span', {}, _('Индекс в списке failover_proxy_links:')),
				idxInput,
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						var idx = parseInt(idxInput.value, 10) || 0;
						return swapListItem('hybrid-failover', section_id, 'failover_proxy_links', idx, -1)
							.then(function() { location.reload(); });
					})
				}, _('Вверх')),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					'click': ui.createHandlerFn(self, function() {
						var idx = parseInt(idxInput.value, 10) || 0;
						return swapListItem('hybrid-failover', section_id, 'failover_proxy_links', idx, 1)
							.then(function() { location.reload(); });
					})
				}, _('Вниз'))
			]);
		};

		o = s.option(form.ListValue, 'proxy_config_type', _('Тип proxy-конфига'));
		o.value('url', _('Connection URL'));
		o.value('outbound', _('Outbound JSON'));
		o.value('urltest', _('URLTest'));
		o.default = 'url';
		o.depends('connection_type', 'proxy');

		o = s.option(form.TextValue, 'proxy_string', _('Proxy URL'));
		o.rows = 4;
		o.depends('proxy_config_type', 'url');
		o.depends('connection_type', 'proxy');

		o = s.option(form.TextValue, 'outbound_json', _('Outbound JSON'));
		o.rows = 10;
		o.depends('proxy_config_type', 'outbound');
		o.depends('connection_type', 'proxy');

		o = s.option(form.DynamicList, 'urltest_proxy_links', _('URLTest URI'));
		o.depends('proxy_config_type', 'urltest');
		o.depends('connection_type', 'proxy');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'urltest_check_interval', _('URLTest interval'));
		o.placeholder = '30s';
		o.description = _('Duration: 30s, 1m, 5m и т.д.');
		o.default = '30s';
		o.depends('proxy_config_type', 'urltest');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'urltest_tolerance', _('URLTest tolerance (ms)'));
		o.placeholder = '50';
		o.depends('proxy_config_type', 'urltest');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'urltest_testing_url', _('URLTest probe URL'));
		o.placeholder = 'https://www.gstatic.com/generate_204';
		o.default = 'https://www.gstatic.com/generate_204';
		o.depends('proxy_config_type', 'urltest');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Value, 'urltest_idle_timeout', _('URLTest idle timeout'));
		o.placeholder = '5m';
		o.depends('proxy_config_type', 'urltest');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Flag, 'urltest_interrupt_exist_connections', _('Interrupt existing connections'));
		o.depends('proxy_config_type', 'urltest');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Flag, 'enable_udp_over_tcp', _('UDP over TCP (SS/SOCKS)'));
		o.depends('connection_type', 'proxy');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.option(form.Flag, 'domain_resolver_enabled', _('Domain resolver'));
		o.depends('connection_type', 'vpn');

		o = s.option(form.ListValue, 'domain_resolver_dns_type', _('Domain resolver DNS type'));
		o.value('doh', _('DoH'));
		o.value('dot', _('DoT'));
		o.value('udp', _('UDP'));
		o.depends('domain_resolver_enabled', '1');

		o = s.option(form.Value, 'domain_resolver_dns_server', _('Domain resolver DNS server'));
		o.placeholder = '8.8.8.8';
		o.depends('domain_resolver_enabled', '1');

		o = s.option(form.MultiValue, 'community_lists', _('Community lists'));
		o.placeholder = _('Service list');
		for (var key in COMMUNITY_LISTS)
			o.value(key, COMMUNITY_LISTS[key]);

		o = s.option(form.ListValue, 'user_domain_list_type', _('User domain list type'));
		o.value('disabled', _('Disabled'));
		o.value('dynamic', _('Dynamic list'));
		o.value('text', _('Text list'));
		o.default = 'disabled';

		o = s.option(form.DynamicList, 'user_domains', _('User domains'));
		o.placeholder = 'example.com';
		o.depends('user_domain_list_type', 'dynamic');

		o = s.option(form.TextValue, 'user_domains_text', _('User domains (text)'));
		o.rows = 8;
		o.depends('user_domain_list_type', 'text');

		o = s.option(form.ListValue, 'user_subnet_list_type', _('Список подсетей / IP (тип)'));
		o.value('disabled', _('Отключён'));
		o.value('dynamic', _('По одному в списке'));
		o.value('text', _('Текстовый список'));
		o.default = 'disabled';

		o = s.option(form.DynamicList, 'user_subnets', _('Подсети и IP через VPN'));
		o.placeholder = '103.21.244.0/22 или 8.8.8.8';
		o.depends('user_subnet_list_type', 'dynamic');

		o = s.option(form.TextValue, 'user_subnets_text', _('Подсети и IP через VPN (текст)'));
		o.rows = 10;
		o.placeholder = '103.21.244.0/22\n8.8.8.8\n// комментарии через //';
		o.depends('user_subnet_list_type', 'text');

		o = s.option(form.DynamicList, 'local_domain_lists', _('Local domain lists'));
		o.placeholder = '/path/domains.lst';

		o = s.option(form.DynamicList, 'local_subnet_lists', _('Local subnet lists'));
		o.placeholder = '/path/subnets.lst';

		o = s.option(form.DynamicList, 'remote_domain_lists', _('Remote domain lists'));
		o.placeholder = 'https://example.com/domains.srs';

		o = s.option(form.DynamicList, 'remote_subnet_lists', _('Remote subnet lists'));
		o.placeholder = 'https://example.com/subnets.srs';

		o = s.option(form.DynamicList, 'fully_routed_ips', _('Fully routed IPs (nft tproxy)'));
		o.placeholder = '192.168.42.215';
		o.description = _('IP/подсети клиентов LAN: весь их трафик принудительно через sing-box (fully routed).');

		this.map = m;

		self._validateOk = false;
		self._stepResultEl = null;
		self._applyBtnEl = null;

		function setStepResult(text, ok) {
			if (!self._stepResultEl)
				return;
			self._stepResultEl.textContent = text || '';
			self._stepResultEl.style.borderColor = ok ? 'rgba(60,186,84,.5)' : 'rgba(231,76,60,.5)';
			if (self._applyBtnEl)
				self._applyBtnEl.disabled = !ok;
		}

		function runValidateStep() {
			if (!self.map) {
				setStepResult(_('Форма не готова'), false);
				return Promise.resolve(false);
			}
			return self.map.save(false).then(function() {
				return callValidateConfig();
			}).then(function(res) {
				var ok = res && res.ok !== false;
				self._validateOk = ok;
				setStepResult(formatStepOutput(res), ok);
				if (!ok)
					notifyRpcResult(_('Проверка'), res);
				return ok;
			}).catch(function(err) {
				self._validateOk = false;
				setStepResult(String(err.message || err), false);
			});
		}

		function renderActionsPanel() {
			var stepResult = E('div', { 'class': 'hf-step-result' }, _('Нажмите «Проверить» после сохранения формы.'));
			self._stepResultEl = stepResult;
			var applyBtn = E('button', {
				'class': 'btn cbi-button cbi-button-save',
				'disabled': true,
				'click': ui.createHandlerFn(self, function() {
					if (!self._validateOk) {
						ui.addNotification(null, E('p', {}, _('Сначала успешная проверка')), 'warning');
						return Promise.resolve();
					}
					return callApplyConfig().then(function(res) {
						notifyRpcResult(_('Применение'), res);
						if (res && res.ok !== false)
							setStepResult(formatStepOutput(res), true);
						return res;
					});
				})
			}, _('3. Применить'));
			self._applyBtnEl = applyBtn;

			var panel = E('div', { 'class': 'cbi-section hf-mon' }, [
				E('h3', {}, _('Применение конфигурации')),
				E('p', { 'class': 'hint' }, _('1) Сохраните форму LuCI  2) Проверить  3) Применить. «Сохранить и применить» выполняет все шаги. «Проверить» сохраняет форму и проверяет sing-box конфиг.')),
				E('p', { 'class': 'hint' }, [
					E('a', { 'href': L.url('admin/services/hybrid-failover/dashboard') }, _('Открыть обзор failover'))
				]),
				E('div', { 'class': 'hf-mon-stepper' }, [
					E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': ui.createHandlerFn(self, function() {
							return runValidateStep();
						})
					}, _('2. Проверить')),
					applyBtn,
					E('button', {
						'class': 'btn cbi-button cbi-button-apply',
						'click': ui.createHandlerFn(self, function() {
							return self.handleSaveApplyChain();
						})
					}, _('Сохранить и применить')),
					E('button', {
						'class': 'btn cbi-button cbi-button-negative',
						'click': ui.createHandlerFn(self, function() {
							self._validateOk = false;
							setStepResult('', false);
							return callPendingRollback().then(function(res) {
								notifyRpcResult(_('Откат pending'), res);
							});
						})
					}, _('Откатить pending')),
					stepResult,
					E('div', { 'style': 'flex:1 1 100%;display:flex;gap:8px;flex-wrap:wrap;margin-top:8px;' }, [
						E('button', {
							'class': 'btn cbi-button cbi-button-action',
							'click': ui.createHandlerFn(self, function() {
								return self.handleRpc(callListUpdate, _('list-update'));
							})
						}, _('Обновить community lists')),
						E('button', {
							'class': 'btn cbi-button cbi-button-action',
							'click': ui.createHandlerFn(self, function() {
								return self.handleRpc(callSubscriptionRefresh, _('subscription-refresh'));
							})
						}, _('Обновить подписки')),
						E('button', {
							'class': 'btn cbi-button cbi-button-neutral',
							'click': ui.createHandlerFn(self, function() {
								var fromInput = E('input', { 'class': 'cbi-input-text', 'value': 'glob', 'style': 'width:100%;margin-bottom:8px;' });
								var toInput = E('input', { 'class': 'cbi-input-text', 'style': 'width:100%;' });
								hfui.showModal(_('Дублировать секцию'), [
									E('label', {}, _('Исходная секция')),
									fromInput,
									E('label', { 'style': 'margin-top:8px;display:block;' }, _('Имя новой секции')),
									toInput
								], function() {
									var from = fromInput.value.trim();
									var to = toInput.value.trim();
									if (!from || !to)
										return;
									callDuplicateSection(from, to).then(function(res) {
										notifyRpcResult(_('Дублирование'), res);
										if (res && res.ok !== false)
											location.reload();
									});
								});
								return Promise.resolve();
							})
						}, _('Дублировать секцию…'))
					])
				])
			]);
			hfui.injectStyles(panel);
			return panel;
		}

		return m.render().then(function(node) {
			var panel = renderActionsPanel();
			if (node && node.appendChild)
				node.appendChild(panel);
			return node;
		});
	},

	handleSaveApply: function() {
		var map = this.map;
		var self = this;
		return map.save(true).then(function() {
			scheduleValidate(self);
		});
	},

	handleSaveApplyChain: function() {
		var self = this;
		var map = this.map;
		if (!map)
			return Promise.resolve();
		return map.save(false).then(function() {
			return callValidateConfig().then(function(vres) {
				var ok = vres && vres.ok !== false;
				self._validateOk = ok;
				if (self._stepResultEl) {
					self._stepResultEl.textContent = formatStepOutput(vres);
					self._stepResultEl.style.borderColor = ok ? 'rgba(60,186,84,.5)' : 'rgba(231,76,60,.5)';
				}
				if (!ok) {
					notifyRpcResult(_('Проверка'), vres);
					return Promise.reject(new Error(_('validate failed')));
				}
				return callApplyConfig();
			}).then(function(res) {
				notifyRpcResult(_('Сохранить и применить'), res);
				if (self._stepResultEl && res && res.ok !== false)
					self._stepResultEl.textContent = formatStepOutput(res);
				return res;
			});
		});
	},

	handleSave: function() {
		var map = this.map;
		var self = this;
		return map.save(false).then(function() {
			scheduleValidate(self);
		});
	}
});
