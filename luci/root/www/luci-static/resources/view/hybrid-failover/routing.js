'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';

var callValidate = rpc.declare({
	object: 'hybrid-failover',
	method: 'pending_validate'
});

var callPendingApply = rpc.declare({
	object: 'hybrid-failover',
	method: 'pending_apply'
});

var callPendingCapture = rpc.declare({
	object: 'hybrid-failover',
	method: 'pending_capture'
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

var _validateTimer = null;

function scheduleValidate(self) {
	if (_validateTimer)
		clearTimeout(_validateTimer);
	_validateTimer = setTimeout(function() {
		callValidate().then(function(res) {
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
	var text = (res && res.output) ? res.output :
		(res && res.ok === false) ? _('Ошибка') :
		(res && res.data) ? JSON.stringify(res.data, null, 2) :
		_('Готово');

	ui.addNotification(null, E('div', [
		E('strong', {}, title),
		E('pre', { 'style': 'white-space:pre-wrap;margin:8px 0 0;' }, text)
	]), res && res.ok === false ? 'danger' : 'info');
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
		o.value('fastest', _('fastest: всегда самый быстрый канал (urltest)'));
		o.default = 'outage-only';
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
			var out = E('pre', {
				'id': 'hf-uri-preview-out-' + section_id,
				'style': 'white-space:pre-wrap;font-size:12px;margin:0;'
			}, '-');
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
						if (d.summary)
							out.textContent = d.summary;
						else if (d.error)
							out.textContent = d.error;
						else
							out.textContent = JSON.stringify(d, null, 2);
					}).catch(function(err) {
						out.textContent = String(err.message || err);
					});
				})
			}, _('Проверить URI'));
			wrap.appendChild(input);
			wrap.appendChild(btn);
			wrap.appendChild(out);
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

		o = s.option(form.ListValue, 'urltest_check_interval', _('URLTest interval'));
		o.value('30s', '30s');
		o.value('1m', '1m');
		o.value('3m', '3m');
		o.value('5m', '5m');
		o.default = '3m';
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

		function renderActionsPanel() {
			return E('div', { 'class': 'cbi-section' }, [
				E('h3', {}, _('Применение конфигурации')),
				E('p', { 'class': 'hint' }, _('Сначала сохраните форму, затем проверьте и примените изменения через core.')),
				E('div', { 'style': 'display:flex;gap:8px;flex-wrap:wrap;' }, [
					E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': ui.createHandlerFn(self, function() {
							return self.handleRpc(callValidate, _('Проверка'));
						})
					}, _('Проверить')),
					E('button', {
						'class': 'btn cbi-button cbi-button-save',
						'click': ui.createHandlerFn(self, function() {
							return self.handleRpc(callPendingApply, _('Применение pending'));
						})
					}, _('Применить')),
					E('button', {
						'class': 'btn cbi-button cbi-button-negative',
						'click': ui.createHandlerFn(self, function() {
							return self.handleRpc(callPendingRollback, _('Откат pending'));
						})
					}, _('Откатить pending')),
					E('button', {
						'class': 'btn cbi-button cbi-button-neutral',
						'click': ui.createHandlerFn(self, function() {
							var from = prompt(_('Исходная секция (например glob)'), 'glob');
							if (!from)
								return Promise.resolve();
							var to = prompt(_('Имя новой секции'));
							if (!to)
								return Promise.resolve();
							return callDuplicateSection(from, to).then(function(res) {
								notifyRpcResult(_('Дублирование'), res);
								if (res && res.ok !== false)
									location.reload();
							}).catch(function(err) {
								ui.addNotification(null, E('p', {}, String(err.message || err)), 'danger');
							});
						})
					}, _('Дублировать секцию…'))
				])
			]);
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
			var cap = callPendingCapture();
			return (cap && typeof cap.then === 'function') ? cap : Promise.resolve();
		});
	},

	handleSave: function() {
		var map = this.map;
		var self = this;
		return map.save(false).then(function() {
			scheduleValidate(self);
			var cap = callPendingCapture();
			return (cap && typeof cap.then === 'function') ? cap : Promise.resolve();
		});
	}
});
