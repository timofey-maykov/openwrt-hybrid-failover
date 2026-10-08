'use strict';
'require view';
'require form';
'require uci';
'require rpc';
'require ui';
'require dom';
'require hybrid-failover.hf-ui as hfui';

var callValidateConfig = rpc.declare({
	object: 'hybrid-failover',
	method: 'validate'
});

var callApplyConfig = rpc.declare({
	object: 'hybrid-failover',
	method: 'apply'
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

var ROUTING_CSS = [
	'.hf-chl { width: 100%; }',
	'.cbi-value:has(.hf-chl) { display: block; }',
	'.cbi-value:has(.hf-chl) > .cbi-value-title { display: block; width: auto; float: none; text-align: left; margin: 0 0 10px; padding: 0; }',
	'.cbi-value:has(.hf-chl) > .cbi-value-field { display: block; width: auto; margin: 0; padding: 0; }',
	'.hf-chl__list { display: flex; flex-direction: column; gap: 8px; }',
	'.hf-chl__empty { padding: 18px; text-align: center; font-size: 13px; color: var(--hf-muted); border: 1px dashed var(--hf-line); border-radius: 10px; }',
	'.hf-chl__item { border: 1px solid var(--hf-line); border-radius: 10px; background: var(--nb-surface, transparent); transition: border-color .15s, box-shadow .15s; }',
	'.hf-chl__item:hover { border-color: var(--nb-border-strong, rgba(127,127,127,.45)); }',
	'.hf-chl__item--active { border-color: var(--hf-ok); box-shadow: inset 3px 0 0 var(--hf-ok); }',
	'.hf-chl__item--drag { opacity: .45; }',
	'.hf-chl__item--over { box-shadow: 0 -2px 0 var(--hf-acc); }',
	'.hf-chl__row { display: flex; align-items: center; gap: 10px; padding: 10px 12px; }',
	'.hf-chl__grip { cursor: grab; color: var(--hf-muted); font-size: 15px; line-height: 1; user-select: none; padding: 2px; }',
	'.hf-chl__num { flex: none; width: 22px; height: 22px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 700; background: var(--hf-surface); border: 1px solid var(--hf-line); }',
	'.hf-chl__proto { flex: none; padding: 3px 8px; border-radius: 6px; font-size: 11px; font-weight: 700; letter-spacing: .03em; color: var(--hf-acc); background: var(--hf-acc-soft); }',
	'.hf-chl__main { flex: 1 1 auto; min-width: 0; }',
	'.hf-chl__name { font-size: 13.5px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }',
	'.hf-chl__meta { display: flex; flex-wrap: wrap; gap: 4px 10px; margin-top: 2px; font-size: 12px; color: var(--hf-muted); }',
	'.hf-chl__meta code { font: 12px var(--hf-mono); background: none; padding: 0; color: inherit; }',
	'.hf-chl__live { flex: none; display: inline-flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 600; white-space: nowrap; }',
	'.hf-chl__live::before { content: ""; width: 8px; height: 8px; border-radius: 50%; background: var(--hf-muted); }',
	'.hf-chl__live--ok { color: var(--hf-ok); } .hf-chl__live--ok::before { background: var(--hf-ok); }',
	'.hf-chl__live--slow { color: var(--hf-warn); } .hf-chl__live--slow::before { background: var(--hf-warn); }',
	'.hf-chl__live--bad { color: var(--hf-bad); } .hf-chl__live--bad::before { background: var(--hf-bad); }',
	'.hf-chl__tools { flex: none; display: flex; gap: 2px; }',
	'.hf-chl__btn { appearance: none; border: 0; background: transparent; color: var(--hf-muted); width: 30px; height: 30px; border-radius: 7px; cursor: pointer; font-size: 15px; line-height: 1; display: inline-flex; align-items: center; justify-content: center; }',
	'.hf-chl__btn:hover { background: var(--nb-hover, rgba(127,127,127,.12)); color: inherit; }',
	'.hf-chl__btn:disabled { opacity: .3; cursor: default; background: transparent; }',
	'.hf-chl__btn--del:hover { color: var(--hf-bad); background: var(--hf-bad-soft); }',
	'.hf-chl__raw { display: none; padding: 0 12px 12px; }',
	'.hf-chl__item--open .hf-chl__raw { display: block; }',
	'.hf-chl__raw textarea { width: 100%; min-height: 70px; box-sizing: border-box; font: 12px/1.45 var(--hf-mono); word-break: break-all; }',
	'.hf-chl__rawbar { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; }',
	'.hf-chl__add { margin-top: 10px; display: flex; gap: 8px; align-items: flex-start; }',
	'.hf-chl__add textarea { flex: 1 1 auto; min-height: 40px; height: 40px; resize: vertical; box-sizing: border-box; font: 12px/1.45 var(--hf-mono); }',
	'.hf-chl__add .btn { flex: none; }',
	'.hf-chl__summary { margin: 0 0 8px; font-size: 12px; color: var(--hf-muted); }',
	'.hf-chips { display: flex; flex-wrap: wrap; gap: 6px; }',
	'.hf-chip { appearance: none; display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; border: 1px solid var(--hf-line); background: transparent; color: inherit; font: inherit; font-size: 12.5px; cursor: pointer; transition: background .12s, border-color .12s; }',
	'.hf-chip:hover { border-color: var(--nb-border-strong, rgba(127,127,127,.5)); }',
	'.hf-chip::before { content: "+"; font-weight: 700; opacity: .5; }',
	'.hf-chip--on { color: var(--hf-acc); background: var(--hf-acc-soft); border-color: var(--hf-acc); font-weight: 600; }',
	'.hf-chip--on::before { content: "✓"; opacity: 1; }',
	'.hf-chips__count { flex: 1 1 100%; margin: 0 0 4px; font-size: 12px; color: var(--hf-muted); }',
	'@media (max-width: 640px) { .hf-chl__row { flex-wrap: wrap; } .hf-chl__main { flex-basis: calc(100% - 90px); } .hf-chl__live { margin-left: 32px; } .hf-chl__tools { margin-left: auto; } }'
].join('\n');

var parseLink = hfui.parseLink;
var linkFacts = hfui.linkFacts;

// Name the engine would generate for a link (channels.DefaultName in core).
function defaultChannelName(p) {
	var scheme = { hy2: 'hysteria2', awg2: 'AWG' }[p.scheme] || p.scheme;
	return scheme + ' ' + p.host;
}

// Matches a link to the engine channel it became. Links of a URLTest group
// are numbered in order (<section>-<n>-out); other lists are matched by the
// server address that the engine puts into the channel display name.
function findLiveChannel(live, section, option, idx, p) {
	if (!live || !live.length)
		return null;
	if (option === 'urltest_proxy_links') {
		var tag = section + '-' + (idx + 1) + '-out';
		for (var i = 0; i < live.length; i++)
			if (live[i].name === tag)
				return live[i];
	}
	if (!p.host)
		return null;
	var needle = p.host + (p.port ? ':' + p.port : '');
	for (var j = 0; j < live.length; j++) {
		var d = String(live[j].display || '');
		if (String(live[j].name || '').indexOf(section + '-') === 0 && d.indexOf(needle) >= 0)
			return live[j];
	}
	return null;
}

function liveBadge(ch) {
	if (!ch)
		return null;
	if (ch.available === false || !(ch.delay_ms > 0))
		return E('span', { 'class': 'hf-chl__live hf-chl__live--bad', 'title': _('Последняя проверка не прошла') }, _('нет связи'));
	var ms = Math.round(ch.delay_ms);
	return E('span', {
		'class': 'hf-chl__live ' + (ms > 600 ? 'hf-chl__live--slow' : 'hf-chl__live--ok'),
		'title': _('Задержка по последней проверке')
	}, ms + ' ' + _('мс'));
}

function showDecoded(uri) {
	return callDecodeURI(uri).then(function(res) {
		var d = res.data || res;
		var text = d.summary || d.error || JSON.stringify(d, null, 2);
		hfui.showModal(_('Разбор ссылки'), [
			E('pre', { 'style': 'white-space:pre-wrap;font-size:12px;margin:0;max-height:320px;overflow:auto;' }, text)
		]);
	}).catch(function(err) {
		hfui.showModal(_('Разбор ссылки'), [ E('p', {}, String(err.message || err)) ]);
	});
}

var ChannelListWidget = ui.AbstractElement.extend({
	__init__: function(values, options) {
		this.values = L.toArray(values).filter(function(v) { return String(v).trim() !== ''; });
		this.options = Object.assign({}, options);
		this.open = {};
		// Bind live status to the link itself while the saved order still
		// matches the running engine, so reordering keeps each delay in place.
		this.liveByLink = {};
		this.nameByLink = {};
		var routes = this.options.routes || [];
		for (var i = 0; i < this.values.length; i++) {
			var link = this.values[i];
			var p = parseLink(link);
			var rc = routes[i];
			var ch = null;
			if (rc && rc.tag)
				(this.options.live || []).forEach(function(c) { if (c.name === rc.tag) ch = c; });
			if (!ch)
				ch = findLiveChannel(this.options.live, this.options.section, this.options.option, i, p);
			if (ch)
				this.liveByLink[link] = ch;
			if (rc && rc.name && rc.name !== defaultChannelName(p))
				this.nameByLink[link] = rc.name;
		}
	},

	render: function() {
		var self = this;
		this.listEl = E('div', { 'class': 'hf-chl__list' });
		this.summaryEl = E('p', { 'class': 'hf-chl__summary' });
		var addArea = E('textarea', {
			'class': 'cbi-input-textarea',
			'rows': 1,
			'spellcheck': 'false',
			'placeholder': _('Вставьте ссылку. Несколько ссылок можно вставить сразу, каждую с новой строки.')
		});
		var addBtn = E('button', {
			'class': 'btn cbi-button cbi-button-add',
			'click': function(ev) {
				ev.preventDefault();
				var added = self.addLinks(addArea.value);
				if (added)
					addArea.value = '';
			}
		}, _('Добавить'));
		addArea.addEventListener('keydown', function(ev) {
			if (ev.key === 'Enter' && !ev.shiftKey) {
				ev.preventDefault();
				addBtn.click();
			}
		});

		var node = E('div', { 'id': this.options.id, 'class': 'hf-chl' }, [
			this.summaryEl,
			this.listEl,
			E('div', { 'class': 'hf-chl__add' }, [ addArea, addBtn ])
		]);
		this.node = node;
		this.setUpdateEvents(node, 'hf-chl-change');
		this.setChangeEvents(node, 'hf-chl-change');
		dom.bindClassInstance(node, this);
		this.redraw();
		return node;
	},

	changed: function() {
		this.redraw();
		this.node.dispatchEvent(new CustomEvent('hf-chl-change', { bubbles: true }));
	},

	addLinks: function(text) {
		var self = this, n = 0;
		String(text || '').split(/\s+/).forEach(function(l) {
			l = l.trim();
			if (!l || self.values.indexOf(l) >= 0)
				return;
			if (!/^[a-z0-9+.-]+:\/\//i.test(l)) {
				ui.addNotification(null, E('p', {}, _('Не похоже на ссылку канала: ') + l.slice(0, 60)), 'warning');
				return;
			}
			self.values.push(l);
			n++;
		});
		if (n)
			this.changed();
		return n > 0;
	},

	move: function(from, to) {
		if (to < 0 || to >= this.values.length || from === to)
			return;
		var v = this.values.splice(from, 1)[0];
		this.values.splice(to, 0, v);
		this.open = {};
		this.changed();
	},

	remove: function(idx) {
		this.values.splice(idx, 1);
		this.open = {};
		this.changed();
	},

	renderItem: function(link, idx) {
		var self = this;
		var opts = this.options;
		var p = parseLink(link);
		var live = this.liveByLink[link] || null;
		var facts = linkFacts(p);
		var count = this.values.length;

		var addr = p.host ? p.host + (p.port ? ':' + p.port : '') : '';
		var title = this.nameByLink[link] || p.name || addr || (p.scheme === 'vpn' ? _('Конфигурация Amnezia') : _('Канал'));
		var meta = [];
		if (title !== addr && addr)
			meta.push(E('code', {}, addr));
		facts.forEach(function(f) { meta.push(E('span', {}, f)); });

		var rawArea = E('textarea', { 'class': 'cbi-input-textarea', 'spellcheck': 'false' }, link);

		var item = E('div', {
			'class': 'hf-chl__item' + (live && live.selected ? ' hf-chl__item--active' : '') + (this.open[idx] ? ' hf-chl__item--open' : ''),
			'draggable': count > 1 ? 'true' : null,
			'data-idx': idx
		}, [
			E('div', { 'class': 'hf-chl__row' }, [
				count > 1 ? E('span', { 'class': 'hf-chl__grip', 'title': _('Перетащите, чтобы поменять порядок') }, '⋮⋮') : '',
				E('span', { 'class': 'hf-chl__num' }, String(idx + 1)),
				E('span', { 'class': 'hf-chl__proto' }, p.proto),
				E('div', { 'class': 'hf-chl__main' }, [
					E('div', { 'class': 'hf-chl__name', 'title': title }, title),
					meta.length ? E('div', { 'class': 'hf-chl__meta' }, meta) : ''
				]),
				liveBadge(live) || '',
				E('div', { 'class': 'hf-chl__tools' }, [
					E('button', {
						'class': 'hf-chl__btn', 'title': _('Выше'), 'disabled': idx === 0 ? '' : null,
						'click': function(ev) { ev.preventDefault(); self.move(idx, idx - 1); }
					}, '↑'),
					E('button', {
						'class': 'hf-chl__btn', 'title': _('Ниже'), 'disabled': idx === count - 1 ? '' : null,
						'click': function(ev) { ev.preventDefault(); self.move(idx, idx + 1); }
					}, '↓'),
					E('button', {
						'class': 'hf-chl__btn', 'title': _('Показать или изменить ссылку'),
						'click': function(ev) {
							ev.preventDefault();
							self.open[idx] = !self.open[idx];
							item.classList.toggle('hf-chl__item--open', !!self.open[idx]);
						}
					}, '✎'),
					E('button', {
						'class': 'hf-chl__btn hf-chl__btn--del', 'title': _('Удалить'),
						'click': function(ev) { ev.preventDefault(); self.remove(idx); }
					}, '✕')
				])
			]),
			E('div', { 'class': 'hf-chl__raw' }, [
				rawArea,
				E('div', { 'class': 'hf-chl__rawbar' }, [
					E('button', {
						'class': 'btn cbi-button cbi-button-save',
						'click': function(ev) {
							ev.preventDefault();
							var v = rawArea.value.trim();
							if (!v)
								return self.remove(idx);
							self.values[idx] = v;
							self.open[idx] = false;
							self.changed();
						}
					}, _('Сохранить ссылку')),
					E('button', {
						'class': 'btn cbi-button cbi-button-action',
						'click': function(ev) { ev.preventDefault(); showDecoded(rawArea.value.trim()); }
					}, _('Разобрать')),
					E('button', {
						'class': 'btn cbi-button cbi-button-neutral',
						'click': function(ev) {
							ev.preventDefault();
							rawArea.select();
							if (navigator.clipboard)
								navigator.clipboard.writeText(rawArea.value).catch(function() {});
						}
					}, _('Копировать'))
				])
			])
		]);

		if (count > 1) {
			item.addEventListener('dragstart', function(ev) {
				if (ev.target !== item)
					return;
				self.dragFrom = idx;
				item.classList.add('hf-chl__item--drag');
				ev.dataTransfer.effectAllowed = 'move';
				ev.dataTransfer.setData('text/plain', String(idx));
			});
			item.addEventListener('dragend', function() { item.classList.remove('hf-chl__item--drag'); });
			item.addEventListener('dragover', function(ev) {
				if (self.dragFrom == null)
					return;
				ev.preventDefault();
				item.classList.add('hf-chl__item--over');
			});
			item.addEventListener('dragleave', function() { item.classList.remove('hf-chl__item--over'); });
			item.addEventListener('drop', function(ev) {
				ev.preventDefault();
				item.classList.remove('hf-chl__item--over');
				var from = self.dragFrom;
				self.dragFrom = null;
				if (from != null)
					self.move(from, idx);
			});
		}
		return item;
	},

	redraw: function() {
		var self = this;
		var items = this.values.map(function(v, i) { return self.renderItem(v, i); });
		if (!items.length)
			items = [ E('div', { 'class': 'hf-chl__empty' }, this.options.emptyText || _('Каналов пока нет. Вставьте ссылку ниже.')) ];
		dom.content(this.listEl, items);

		var n = this.values.length;
		var up = 0;
		this.values.forEach(function(v, i) {
			var ch = self.liveByLink[v];
			if (ch && ch.available !== false && ch.delay_ms > 0)
				up++;
		});
		var text = n ? (_('Каналов: ') + n) : '';
		if (n && this.options.live && this.options.live.length)
			text += ', ' + _('на связи: ') + up;
		if (n > 1 && this.options.orderHint)
			text += '. ' + this.options.orderHint;
		this.summaryEl.textContent = text;
		this.summaryEl.style.display = text ? '' : 'none';
	},

	getValue: function() {
		return this.values.slice();
	},

	setValue: function(v) {
		this.values = L.toArray(v);
		this.redraw();
	}
});

var ChipsWidget = ui.AbstractElement.extend({
	__init__: function(values, choices, options) {
		this.values = L.toArray(values);
		this.choices = choices;
		this.options = Object.assign({}, options);
	},

	render: function() {
		var self = this;
		this.countEl = E('p', { 'class': 'hf-chips__count' });
		var chips = Object.keys(this.choices).map(function(key) {
			var chip = E('button', {
				'class': 'hf-chip' + (self.values.indexOf(key) >= 0 ? ' hf-chip--on' : ''),
				'aria-pressed': self.values.indexOf(key) >= 0 ? 'true' : 'false',
				'click': function(ev) {
					ev.preventDefault();
					var i = self.values.indexOf(key);
					if (i >= 0)
						self.values.splice(i, 1);
					else
						self.values.push(key);
					chip.classList.toggle('hf-chip--on', i < 0);
					chip.setAttribute('aria-pressed', i < 0 ? 'true' : 'false');
					self.updateCount();
					self.node.dispatchEvent(new CustomEvent('hf-chips-change', { bubbles: true }));
				}
			}, self.choices[key]);
			return chip;
		});
		var node = E('div', { 'id': this.options.id, 'class': 'hf-chips' }, [ this.countEl ].concat(chips));
		this.node = node;
		this.setUpdateEvents(node, 'hf-chips-change');
		this.setChangeEvents(node, 'hf-chips-change');
		dom.bindClassInstance(node, this);
		this.updateCount();
		return node;
	},

	updateCount: function() {
		this.countEl.textContent = this.values.length ? (_('Выбрано: ') + this.values.length) : _('Ничего не выбрано');
	},

	getValue: function() {
		return this.values.slice();
	},

	setValue: function(v) {
		this.values = L.toArray(v);
	}
});

var CBIChips = form.MultiValue.extend({
	__name__: 'CBI.HFChips',

	renderWidget: function(section_id, option_index, cfgvalue) {
		var choices = {};
		for (var i = 0; i < this.keylist.length; i++)
			choices[this.keylist[i]] = this.vallist[i];
		return new ChipsWidget(cfgvalue != null ? cfgvalue : this.default, choices, {
			id: this.cbid(section_id)
		}).render();
	}
});

var CBIChannelList = form.DynamicList.extend({
	__name__: 'CBI.HFChannelList',

	renderWidget: function(section_id, option_index, cfgvalue) {
		var widget = new ChannelListWidget(cfgvalue != null ? cfgvalue : this.default, {
			id: this.cbid(section_id),
			section: section_id,
			option: this.option,
			live: this.live,
			routes: (this.routes || {})[section_id],
			orderHint: this.orderHint,
			emptyText: this.emptyText
		});
		return widget.render();
	}
});

function formatStepOutput(res) {
	if (!res || res.ok === false)
		return (res && (res.output || res.error)) ? String(res.output || res.error) : _('Ошибка');
	if (res.output)
		return String(res.output);
	if (res.data != null)
		return JSON.stringify(res.data, null, 2);
	return _('Готово');
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
		return Promise.all([
			uci.load('hybrid-failover'),
			L.resolveDefault(hfui.rpc.status(), null),
			L.resolveDefault(hfui.rpc.listRoutes(), null)
		]);
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

	renderHeader: function(status, names) {
		var self = this;
		var data = status ? hfui.unwrapData(status) : null;
		var pills = [];

		if (!data) {
			pills.push(hfui.pill(_('Состояние недоступно'), ''));
		} else {
			var running = data.engine_running || data.singbox_running;
			pills.push(hfui.pill(running ? _('Движок работает') : _('Движок остановлен'), running ? 'ok' : 'bad'));
			var tag = hfui.activeChannelTag(data);
			if (tag) {
				var active = null;
				(data.channels || []).forEach(function(c) { if (c.name === tag) active = c; });
				pills.push(hfui.pill(_('Сейчас через: ') + hfui.tagTitle(tag, data, names) +
					(active && active.delay_ms > 0 ? ' · ' + Math.round(active.delay_ms) + ' ' + _('мс') : ''), 'plain'));
			}
			if (data.meta && data.meta.core_version)
				pills.push(hfui.pill('v' + data.meta.core_version, 'plain'));
		}

		this.header = hfui.pageHeader({
			title: _('Маршрутизация'),
			pills: pills,
			actions: [
				hfui.moreMenu(_('Ещё ▾'), [
					{ label: _('Проверить сохранённую конфигурацию'), fn: function() { return self.runValidateStep(); } },
					{ label: _('Обновить списки сервисов'), fn: function() { return self.handleRpc(callListUpdate, _('Обновление списков')); } },
					{ label: _('Обновить подписки'), fn: function() { return self.handleRpc(callSubscriptionRefresh, _('Обновление подписок')); } },
					{ label: _('Копировать секцию…'), fn: function() { return self.handleDuplicate(); } }
				]),
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, function() { return this.handleSaveApplyChain(); })
				}, _('Сохранить и применить'))
			],
			hint: _('Изменения вступают в силу после «Сохранить и применить». Если после применения связь с роутером пропадёт, LuCI сам вернёт прошлые настройки.')
		});
		return this.header;
	},

	setStepResult: function(text, ok) {
		if (this.header)
			this.header.setResult(text, ok);
	},

	runValidateStep: function() {
		var self = this;
		if (!this.map)
			return Promise.resolve(false);
		return this.map.save(false).then(function() {
			return callValidateConfig();
		}).then(function(res) {
			var ok = res && res.ok !== false;
			self.setStepResult(ok ? _('Сохранённая конфигурация в порядке.') + '\n' + formatStepOutput(res) : formatStepOutput(res), ok);
			if (!ok)
				notifyRpcResult(_('Проверка'), res);
			return ok;
		}).catch(function(err) {
			self.setStepResult(String(err.message || err), false);
		});
	},

	handleDuplicate: function() {
		var fromInput = E('input', { 'class': 'cbi-input-text', 'value': uci.get('hybrid-failover', 'settings', 'main_section') || 'glob', 'style': 'width:100%;margin-bottom:8px;' });
		var toInput = E('input', { 'class': 'cbi-input-text', 'style': 'width:100%;', 'placeholder': 'work' });
		hfui.showModal(_('Копировать секцию'), [
			E('label', {}, _('Какую секцию копировать')),
			fromInput,
			E('label', { 'style': 'margin-top:8px;display:block;' }, _('Имя новой секции (латиница, без пробелов)')),
			toInput
		], function() {
			var from = fromInput.value.trim();
			var to = toInput.value.trim();
			if (!from || !to)
				return;
			callDuplicateSection(from, to).then(function(res) {
				notifyRpcResult(_('Копирование секции'), res);
				if (res && res.ok !== false)
					location.reload();
			});
		});
		return Promise.resolve();
	},

	render: function(loaded) {
		var status = loaded[1];
		var data = status ? hfui.unwrapData(status) : null;
		var live = data && Array.isArray(data.channels) ? data.channels : [];
		var routesData = loaded[2] ? hfui.unwrapData(loaded[2]) : null;
		var names = hfui.channelNamesFrom(loaded[2]);
		var routes = {};
		((routesData && routesData.sections) || []).forEach(function(sec) {
			routes[sec.name] = (sec.channels || []).filter(function(c) { return !c.primary; });
		});
		var m, st, s, o;

		m = new form.Map('hybrid-failover');

		/* Global settings */

		st = m.section(form.NamedSection, 'settings', 'settings', _('Общие настройки'));
		st.tab('main', _('Основное'));
		st.tab('dns', _('DNS'));
		st.tab('subs', _('Подписки'));
		st.tab('lists', _('Списки'));
		st.tab('clash', _('Clash API'));
		st.tab('extra', _('Дополнительно'));

		o = st.taboption('main', form.Flag, 'enabled', _('Hybrid Failover включён'));
		o.default = '1';

		o = st.taboption('main', form.Flag, 'disable_quic', _('Отключить QUIC'));
		o.description = _('Приложения перейдут с QUIC на обычный HTTPS. Через туннель так обычно стабильнее.');

		o = st.taboption('main', form.Flag, 'disable_lan_ipv6', _('Отключить IPv6 в домашней сети'));
		o.default = '1';
		o.description = _('Выключает RA и DHCPv6 и убирает AAAA-ответы в dnsmasq. Без этого Instagram и другие сервисы могут уходить мимо туннеля по IPv6. «Частный DNS» на телефоне всё равно нужно выключить вручную.');

		o = st.taboption('main', form.Flag, 'dont_touch_dhcp', _('Не менять настройки dnsmasq и DHCP'));

		o = st.taboption('main', form.DynamicList, 'routing_excluded_ips', _('Устройства в обход'));
		o.description = _('Трафик этих адресов никогда не идёт в туннель. Отдельные правила для устройств на вкладке «Клиенты».');
		o.placeholder = '192.168.1.100';

		o = st.taboption('dns', form.ListValue, 'dns_type', _('Протокол DNS'));
		o.value('doh', _('DNS over HTTPS (DoH)'));
		o.value('dot', _('DNS over TLS (DoT)'));
		o.value('udp', _('Обычный DNS (UDP)'));

		o = st.taboption('dns', form.Value, 'dns_server', _('DNS-сервер'));
		o.placeholder = '1.1.1.1';

		o = st.taboption('dns', form.Value, 'bootstrap_dns_server', _('Bootstrap DNS'));
		o.placeholder = '77.88.8.8';
		o.description = _('Нужен, чтобы найти сам DNS-сервер, если он задан именем.');

		o = st.taboption('dns', form.Value, 'dns_rewrite_ttl', _('Время жизни DNS-ответов, сек'));
		o.placeholder = '60';
		o.datatype = 'uinteger';

		o = st.taboption('subs', form.DynamicList, 'subscription_urls', _('Ссылки на подписки'));
		o.placeholder = 'https://…';
		o.description = _('Каналы из подписки появляются в списке каналов основной секции.');

		o = st.taboption('subs', form.ListValue, 'subscription_update_interval', _('Обновлять подписки'));
		o.value('off', _('вручную'));
		o.value('1h', _('раз в час'));
		o.value('3h', _('каждые 3 часа'));
		o.value('6h', _('каждые 6 часов'));
		o.value('12h', _('каждые 12 часов'));
		o.value('1d', _('раз в сутки'));
		o.default = 'off';
		o.description = _('Заменяются только каналы из подписки. Добавленные вручную остаются.');

		o = st.taboption('lists', form.Value, 'update_interval', _('Как часто обновлять списки сервисов'));
		o.placeholder = '1d';
		o.description = _('Например 12h или 1d.');

		o = st.taboption('lists', form.Flag, 'download_lists_via_proxy', _('Скачивать списки через туннель'));

		o = st.taboption('lists', form.Value, 'download_lists_via_proxy_section', _('Через какую секцию скачивать'));
		o.placeholder = 'glob';
		o.depends('download_lists_via_proxy', '1');

		o = st.taboption('clash', form.Value, 'clash_api_listen', _('Адрес Clash API'));
		o.placeholder = '192.168.1.1:9090';

		o = st.taboption('clash', form.Flag, 'enable_yacd', _('Веб-панель Yacd'));

		o = st.taboption('clash', form.DummyValue, '_yacd_link', _('Открыть панель'));
		o.depends('enable_yacd', '1');
		o.renderWidget = function() {
			var listen = String(uci.get('hybrid-failover', 'settings', 'clash_api_listen') || '127.0.0.1:9090').trim();
			var url = listen.indexOf('://') >= 0 ? listen : ('http://' + listen);
			if (url.slice(-1) !== '/')
				url += '/';
			url += 'ui';
			return E('a', { 'href': url, 'target': '_blank', 'rel': 'noopener' }, url);
		};

		o = st.taboption('clash', form.Flag, 'enable_yacd_wan_access', _('Доступ из интернета'));
		o.depends('enable_yacd', '1');
		o.description = _('API будет слушать 0.0.0.0 и станет доступен снаружи. Не включайте без необходимости.');

		o = st.taboption('clash', form.Value, 'yacd_secret_key', _('Секрет API'));
		o.password = true;
		o.depends('enable_yacd', '1');

		o = st.taboption('extra', form.Value, 'main_section', _('Основная секция'));
		o.placeholder = 'glob';

		o = st.taboption('extra', form.Value, 'output_network_interface', _('Исходящий интерфейс'));
		o.placeholder = _('автоматически');

		o = st.taboption('extra', form.Value, 'failover_probe_interval', _('Как часто проверять основной VPN'));
		o.placeholder = '30s';
		o.description = _('Для режима VPN с резервными каналами. Проверка самих каналов настраивается в секции.');

		o = st.taboption('extra', form.Value, 'webhook_url', _('Webhook при переключении'));
		o.placeholder = 'https://…';

		o = st.taboption('extra', form.Value, 'history_max_lines', _('Строк в журнале переключений'));
		o.placeholder = '500';
		o.datatype = 'uinteger';

		o = st.taboption('extra', form.Value, 'delay_history_points', _('Точек на графике задержек'));
		o.placeholder = '50';
		o.datatype = 'uinteger';

		o = st.taboption('extra', form.Value, 'cache_path', _('Файл кэша'));
		o.placeholder = '/etc/sing-box/cache.db';

		/* Routing sections */

		s = m.section(form.TypedSection, 'section', _('Секции маршрутизации'),
			_('Секция решает, какой трафик и через какие каналы идёт. Обычно хватает одной основной секции.'));
		s.anonymous = false;
		s.addremove = true;
		s.addbtntitle = _('Добавить секцию');

		s.tab('conn', _('Подключение'));
		s.tab('channels', _('Каналы'));
		s.tab('check', _('Проверка каналов'));
		s.tab('what', _('Что направлять'));
		s.tab('ext', _('Внешние списки'));
		s.tab('extra', _('Дополнительно'));

		o = s.taboption('conn', form.Flag, 'enabled', _('Секция включена'));
		o.default = '1';

		o = s.taboption('conn', form.ListValue, 'connection_type', _('Через что вести трафик'));
		o.value('proxy', _('Через прокси-каналы'));
		o.value('vpn', _('Через VPN-интерфейс'));
		o.value('block', _('Блокировать'));

		o = s.taboption('conn', form.ListValue, 'proxy_config_type', _('Как заданы каналы'));
		o.value('urltest', _('Несколько ссылок, работает самая быстрая'));
		o.value('url', _('Одна ссылка'));
		o.value('outbound', _('JSON outbound sing-box'));
		o.default = 'url';
		o.depends('connection_type', 'proxy');

		o = s.taboption('conn', form.TextValue, 'proxy_string', _('Ссылка'));
		o.rows = 3;
		o.monospace = true;
		o.depends({ connection_type: 'proxy', proxy_config_type: 'url' });

		o = s.taboption('conn', form.TextValue, 'outbound_json', _('Outbound JSON'));
		o.rows = 10;
		o.monospace = true;
		o.depends({ connection_type: 'proxy', proxy_config_type: 'outbound' });

		o = s.taboption('conn', form.Value, 'interface', _('VPN-интерфейс'));
		o.placeholder = 'awg0';
		o.depends('connection_type', 'vpn');

		o = s.taboption('conn', form.Flag, 'failover_vpn_enabled', _('Резервные каналы, если VPN упал'));
		o.depends('connection_type', 'vpn');

		o = s.taboption('conn', form.ListValue, 'failover_policy', _('Когда переключаться'));
		o.value('outage-only', _('Только когда VPN упал'));
		o.value('prefer-primary', _('Сразу возвращаться на VPN, когда он поднялся'));
		o.value('fastest', _('Всегда через самый быстрый канал'));
		o.default = 'outage-only';
		o.description = _('Подробнее о режимах на странице «Обзор».');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('conn', form.Value, 'failover_fail_threshold', _('Неудачных проверок до переключения'));
		o.placeholder = '2';
		o.datatype = 'uinteger';
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('conn', form.Value, 'failover_recover_threshold', _('Удачных проверок до возврата'));
		o.placeholder = '2';
		o.datatype = 'uinteger';
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('channels', CBIChannelList, 'urltest_proxy_links', _('Каналы'));
		o.live = live;
		o.routes = this.routesFor(routes, 'proxy');
		o.description = _('Каждая ссылка (vless, hysteria2, awg2, trojan, ss, socks) становится отдельным каналом. Движок их проверяет, ведёт трафик через самый быстрый, а при отказе переключается на следующий.');
		o.emptyText = _('Каналов пока нет. Вставьте ссылку ниже или добавьте подписку в общих настройках.');
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('channels', CBIChannelList, 'failover_proxy_links', _('Резервные каналы'));
		o.live = live;
		o.routes = this.routesFor(routes, 'vpn');
		o.orderHint = _('Чем выше канал, тем раньше он используется');
		o.description = _('Используются, когда VPN недоступен. Подходят и ссылки vpn:// и awg2://. Для AmneziaWG 3.1 на роутере нужны kmod-amneziawg и amneziawg-tools 3.1 или новее.');
		o.emptyText = _('Резервных каналов нет. Вставьте ссылку ниже.');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('channels', form.Flag, 'enable_udp_over_tcp', _('UDP поверх TCP для Shadowsocks и SOCKS'));
		o.depends('connection_type', 'proxy');
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('check', form.Value, 'urltest_check_interval', _('Как часто проверять'));
		o.placeholder = '30s';
		o.default = '30s';
		o.description = _('Например 30s, 1m или 5m.');
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('check', form.Value, 'urltest_tolerance', _('Порог переключения, мс'));
		o.placeholder = '50';
		o.datatype = 'uinteger';
		o.description = _('Канал меняется, только если другой быстрее хотя бы на столько. Так трафик не прыгает между почти равными каналами.');
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('check', form.Value, 'urltest_testing_url', _('Адрес для проверки'));
		o.placeholder = 'https://www.gstatic.com/generate_204';
		o.default = 'https://www.gstatic.com/generate_204';
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('check', form.Value, 'urltest_idle_timeout', _('Пауза проверок без трафика'));
		o.placeholder = '5m';
		o.description = _('Если трафика нет так долго, каналы перестают проверяться до первого запроса.');
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('check', form.Flag, 'urltest_interrupt_exist_connections', _('Рвать соединения при смене канала'));
		o.description = _('Открытые соединения сразу переходят на новый канал. Иначе они доживают на старом.');
		o.depends({ connection_type: 'proxy', proxy_config_type: 'urltest' });
		o.depends({ connection_type: 'vpn', failover_vpn_enabled: '1' });

		o = s.taboption('what', CBIChips, 'community_lists', _('Готовые списки сервисов'));
		o.description = _('Домены и адреса из выбранных списков пойдут через эту секцию.');
		for (var key in COMMUNITY_LISTS)
			o.value(key, COMMUNITY_LISTS[key]);

		o = s.taboption('what', form.ListValue, 'user_domain_list_type', _('Свои домены'));
		o.value('disabled', _('Нет'));
		o.value('dynamic', _('Списком'));
		o.value('text', _('Текстом'));
		o.default = 'disabled';

		o = s.taboption('what', form.DynamicList, 'user_domains', _('Домены'));
		o.placeholder = 'example.com';
		o.depends('user_domain_list_type', 'dynamic');

		o = s.taboption('what', form.TextValue, 'user_domains_text', _('Домены'));
		o.rows = 8;
		o.placeholder = 'example.com\nexample.org';
		o.depends('user_domain_list_type', 'text');

		o = s.taboption('what', form.ListValue, 'user_subnet_list_type', _('Свои IP и подсети'));
		o.value('disabled', _('Нет'));
		o.value('dynamic', _('Списком'));
		o.value('text', _('Текстом'));
		o.default = 'disabled';

		o = s.taboption('what', form.DynamicList, 'user_subnets', _('IP и подсети'));
		o.placeholder = '103.21.244.0/22';
		o.depends('user_subnet_list_type', 'dynamic');

		o = s.taboption('what', form.TextValue, 'user_subnets_text', _('IP и подсети'));
		o.rows = 10;
		o.placeholder = '103.21.244.0/22\n8.8.8.8\n// комментарии через //';
		o.depends('user_subnet_list_type', 'text');

		o = s.taboption('what', form.DynamicList, 'fully_routed_ips', _('Устройства целиком через туннель'));
		o.placeholder = '192.168.1.215';
		o.description = _('Весь трафик этих устройств или подсетей идёт через секцию, без разбора по спискам.');

		o = s.taboption('ext', form.DynamicList, 'remote_domain_lists', _('Списки доменов по ссылке'));
		o.placeholder = 'https://example.com/domains.srs';

		o = s.taboption('ext', form.DynamicList, 'remote_subnet_lists', _('Списки подсетей по ссылке'));
		o.placeholder = 'https://example.com/subnets.srs';

		o = s.taboption('ext', form.DynamicList, 'local_domain_lists', _('Файлы доменов на роутере'));
		o.placeholder = '/etc/hybrid-failover/domains.lst';

		o = s.taboption('ext', form.DynamicList, 'local_subnet_lists', _('Файлы подсетей на роутере'));
		o.placeholder = '/etc/hybrid-failover/subnets.lst';

		o = s.taboption('extra', form.Flag, 'domain_resolver_enabled', _('Свой DNS для серверов секции'));
		o.description = _('Адреса серверов VPN будут определяться через указанный DNS.');
		o.depends('connection_type', 'vpn');

		o = s.taboption('extra', form.ListValue, 'domain_resolver_dns_type', _('Протокол DNS'));
		o.value('doh', _('DNS over HTTPS (DoH)'));
		o.value('dot', _('DNS over TLS (DoT)'));
		o.value('udp', _('Обычный DNS (UDP)'));
		o.depends('domain_resolver_enabled', '1');

		o = s.taboption('extra', form.Value, 'domain_resolver_dns_server', _('DNS-сервер'));
		o.placeholder = '8.8.8.8';
		o.depends('domain_resolver_enabled', '1');

		this.map = m;

		var header = this.renderHeader(status, names);
		var style = E('style', { 'type': 'text/css' }, ROUTING_CSS);

		return m.render().then(function(mapEl) {
			var root = E('div', { 'class': 'hf-page hf-rt' }, [ style, header, mapEl ]);
			hfui.injectStyles(root);
			return root;
		});
	},

	// The core reads /etc/config, while form.Map.save() only stages changes
	// in the LuCI session. Staged changes go through the standard LuCI apply
	// (commit with rollback on lost connectivity); the procd reload trigger
	// then rebuilds the engine. With nothing staged, re-apply the saved config.
	handleSaveApplyChain: function() {
		var self = this;
		if (!this.map)
			return Promise.resolve();
		return this.map.save().then(function() {
			return L.resolveDefault(uci.changes(), {});
		}).then(function(changes) {
			var n = 0;
			for (var cfg in changes)
				n += (changes[cfg] || []).length;
			if (n > 0) {
				self.setStepResult(_('Записываю изменения…'), true);
				return ui.changes.apply(true);
			}
			self.setStepResult(_('Изменений нет. Проверяю и применяю сохранённую конфигурацию…'), true);
			return callValidateConfig().then(function(vres) {
				if (!vres || vres.ok === false) {
					self.setStepResult(_('Сохранённая конфигурация с ошибкой, ничего не применено.') + '\n' + formatStepOutput(vres), false);
					return null;
				}
				return callApplyConfig().then(function(res) {
					var good = res && res.ok !== false;
					self.setStepResult((good ? _('Применено.') : _('Не удалось применить.')) + '\n' + formatStepOutput(res), good);
					return res;
				});
			});
		}).catch(function(err) {
			self.setStepResult(String(err.message || err), false);
		});
	},

	routesFor: function(routes, type) {
		var out = {};
		uci.sections('hybrid-failover', 'section').forEach(function(sec) {
			if ((sec.connection_type || '') === type && routes[sec['.name']])
				out[sec['.name']] = routes[sec['.name']];
		});
		return out;
	}
});
