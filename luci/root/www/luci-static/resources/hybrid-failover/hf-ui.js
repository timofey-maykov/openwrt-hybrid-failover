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

// Shared look of all Hybrid Failover pages: page header with live state,
// pills, the "More" menu, result box, panels and chips. Colours come from
// the Nimbus theme tokens with fallbacks for the stock themes.
var HF_PAGE_CSS = [
	'.hf-page, .hf-mon { --hf-acc: var(--nb-accent, #4f46e5); --hf-acc-soft: var(--nb-accent-soft, rgba(79,70,229,.1)); --hf-ok: var(--nb-success, #16a34a); --hf-ok-soft: var(--nb-success-soft, rgba(22,163,74,.1)); --hf-bad: var(--nb-danger, #dc2626); --hf-bad-soft: var(--nb-danger-soft, rgba(220,38,38,.09)); --hf-warn: var(--nb-warning, #d97706); --hf-warn-soft: var(--nb-warning-soft, rgba(217,119,6,.11)); --hf-line: var(--nb-border, rgba(127,127,127,.25)); --hf-surface: var(--nb-surface-2, rgba(127,127,127,.05)); --hf-card: var(--nb-surface, transparent); --hf-muted: var(--nb-text-2, inherit); --hf-mono: var(--font-mono, ui-monospace, Menlo, Consolas, monospace); }',
	'.hf-head { display: flex; flex-wrap: wrap; gap: 16px 24px; align-items: center; justify-content: space-between; padding: 18px 20px; margin-bottom: 18px; border: 1px solid var(--hf-line); border-radius: var(--nb-radius-lg, 14px); background: var(--hf-card); box-shadow: var(--nb-shadow-sm, none); }',
	'.hf-head__info { flex: 1 1 320px; min-width: 0; }',
	'.hf-head__title { margin: 0 0 8px; font-size: 1.25rem; font-weight: 700; letter-spacing: -.01em; }',
	'.hf-head__state { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }',
	'.hf-head__actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }',
	'.hf-head__hint { flex: 1 1 100%; margin: 0; font-size: 12.5px; color: var(--hf-muted); }',
	'.hf-pill { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 600; background: var(--hf-surface); border: 1px solid var(--hf-line); white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }',
	'.hf-pill::before { content: ""; width: 7px; height: 7px; border-radius: 50%; background: var(--hf-muted); flex: none; }',
	'.hf-pill--ok { color: var(--hf-ok); background: var(--hf-ok-soft); border-color: transparent; } .hf-pill--ok::before { background: var(--hf-ok); }',
	'.hf-pill--bad { color: var(--hf-bad); background: var(--hf-bad-soft); border-color: transparent; } .hf-pill--bad::before { background: var(--hf-bad); }',
	'.hf-pill--warn { color: var(--hf-warn); background: var(--hf-warn-soft); border-color: transparent; } .hf-pill--warn::before { background: var(--hf-warn); }',
	'.hf-pill--plain::before { display: none; }',
	'.hf-more { position: relative; }',
	'.hf-more__menu { position: absolute; right: 0; top: calc(100% + 6px); z-index: 30; min-width: 260px; padding: 6px; border-radius: var(--nb-radius, 10px); border: 1px solid var(--hf-line); background: var(--nb-surface, #fff); box-shadow: var(--nb-shadow-lg, 0 10px 30px rgba(0,0,0,.2)); display: none; }',
	'.hf-more--open .hf-more__menu { display: block; }',
	'.hf-more__menu button { display: block; width: 100%; white-space: nowrap; text-align: left; padding: 8px 10px; border: 0; border-radius: 7px; background: transparent; color: inherit; font: inherit; font-size: 13px; cursor: pointer; }',
	'.hf-more__menu button:hover { background: var(--nb-hover, rgba(127,127,127,.1)); }',
	'.hf-more__menu .hf-more__danger { color: var(--hf-bad); }',
	'.hf-result { display: none; margin: 0; padding: 10px 12px; max-height: 260px; overflow: auto; border-radius: 8px; font: 12px/1.5 var(--hf-mono); white-space: pre-wrap; word-break: break-word; background: var(--hf-surface); border: 1px solid var(--hf-line); }',
	'.hf-head .hf-result { flex: 1 1 100%; }',
	'.hf-result--ok { display: block; border-color: var(--hf-ok); }',
	'.hf-result--bad { display: block; border-color: var(--hf-bad); }',
	'.hf-result--info { display: block; }',
	'.hf-panels { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 16px; margin-bottom: 18px; }',
	'.hf-panel { border: 1px solid var(--hf-line); border-radius: var(--nb-radius-lg, 14px); background: var(--hf-card); padding: 18px 20px; box-shadow: var(--nb-shadow-sm, none); min-width: 0; }',
	'.hf-panel__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 12px; }',
	'.hf-panel__title { margin: 0; font-size: 15px; font-weight: 700; }',
	'.hf-panel__sub { margin: 4px 0 0; font-size: 12.5px; color: var(--hf-muted); line-height: 1.45; }',
	'.hf-panel__actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }',
	'.hf-panel--wide { grid-column: 1 / -1; }',
	'.hf-checks { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }',
	'.hf-checks li { display: flex; gap: 10px; align-items: flex-start; padding: 8px 10px; border-radius: 8px; background: var(--hf-surface); font-size: 13px; line-height: 1.4; }',
	'.hf-checks__mark { flex: none; width: 18px; height: 18px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 700; color: #fff; background: var(--hf-ok); }',
	'.hf-checks li.hf-checks--bad .hf-checks__mark { background: var(--hf-bad); }',
	'.hf-checks__empty { font-size: 13px; color: var(--hf-muted); padding: 10px 0; }',
	'.hf-kv { display: grid; grid-template-columns: minmax(110px, 38%) minmax(0, 1fr); gap: 8px 16px; margin: 0; font-size: 13px; }',
	'.hf-kv dt { color: var(--hf-muted); margin: 0; }',
	'.hf-kv dd { margin: 0; font-weight: 600; word-break: break-word; }',
	'.hf-details { margin-top: 14px; font-size: 13px; }',
	'.hf-details > summary { cursor: pointer; color: var(--hf-muted); font-size: 12.5px; }',
	'.hf-details[open] > summary { margin-bottom: 10px; }',
	'.hf-field { display: flex; flex-direction: column; gap: 6px; margin-bottom: 14px; }',
	'.hf-field > label { font-size: 13px; font-weight: 600; }',
	'.hf-field__hint { font-size: 12px; color: var(--hf-muted); line-height: 1.4; }',
	'.hf-field input[type=text], .hf-field input[type=password], .hf-field input[type=number], .hf-field select, .hf-field textarea { width: 100%; box-sizing: border-box; }',
	'.hf-field__row { display: flex; gap: 8px; align-items: center; }',
	'.hf-field__row > input { flex: 1 1 auto; }',
	'.hf-switch-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 0; border-top: 1px solid var(--hf-line); }',
	'.hf-switch-row:first-child { border-top: 0; padding-top: 0; }',
	'.hf-switch-row label { font-size: 13px; font-weight: 600; }',
	'.hf-switch-row .hf-field__hint { margin-top: 2px; font-weight: 400; }',
	'.hf-empty { padding: 22px; text-align: center; font-size: 13px; color: var(--hf-muted); border: 1px dashed var(--hf-line); border-radius: 10px; }',
	'.hf-proto { flex: none; display: inline-block; padding: 2px 7px; border-radius: 6px; font-size: 11px; font-weight: 700; letter-spacing: .03em; color: var(--hf-acc); background: var(--hf-acc-soft); }',
	'.hf-stat { display: flex; flex-direction: column; gap: 4px; padding: 12px 14px; border-radius: 10px; background: var(--hf-surface); border: 1px solid var(--hf-line); min-width: 0; }',
	'.hf-stat__label { font-size: 12px; color: var(--hf-muted); }',
	'.hf-stat__value { font-size: 15px; font-weight: 700; line-height: 1.3; overflow-wrap: anywhere; }',
	'.hf-chname { display: inline-flex; align-items: center; gap: 6px; flex-wrap: wrap; min-width: 0; }',
	'.hf-stat--ok .hf-stat__value { color: var(--hf-ok); } .hf-stat--bad .hf-stat__value { color: var(--hf-bad); } .hf-stat--warn .hf-stat__value { color: var(--hf-warn); }',
	'.hf-stats { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 10px; margin-bottom: 16px; }',
	'@media (max-width: 640px) { .hf-head__actions { width: 100%; } .hf-head__actions > .btn, .hf-head__actions > .hf-more { flex: 1 1 auto; } .hf-head__actions > .hf-more > .btn { width: 100%; } .hf-panel { padding: 14px; } .hf-kv { grid-template-columns: 1fr; gap: 2px; } .hf-kv dd { margin-bottom: 8px; } }'
].join('\n');

var PROTO_LABELS = {
	vless: 'VLESS', vmess: 'VMess', trojan: 'Trojan', ss: 'Shadowsocks', shadowsocks: 'Shadowsocks',
	hy2: 'Hysteria2', hysteria2: 'Hysteria2', hysteria: 'Hysteria', tuic: 'TUIC',
	awg: 'AmneziaWG', awg2: 'AmneziaWG', amneziawg: 'AmneziaWG', wg: 'WireGuard', wireguard: 'WireGuard',
	socks: 'SOCKS', socks5: 'SOCKS', http: 'HTTP', https: 'HTTPS', vpn: 'Amnezia', direct: 'VPN'
};

function protoLabel(type) {
	var t = String(type || '').toLowerCase();
	return PROTO_LABELS[t] || (t ? t.toUpperCase() : '');
}

function safeDecode(s) {
	try {
		return decodeURIComponent(String(s).replace(/\+/g, ' '));
	} catch (e) {
		return String(s);
	}
}

function b64decode(s) {
	try {
		s = String(s).replace(/-/g, '+').replace(/_/g, '/');
		while (s.length % 4)
			s += '=';
		return decodeURIComponent(escape(atob(s)));
	} catch (e) {
		return null;
	}
}

function splitHostPort(hp) {
	hp = String(hp || '').replace(/\/.*$/, '');
	var m = hp.match(/^\[([^\]]+)\](?::(\d+))?$/);
	if (m)
		return { host: m[1], port: m[2] || '' };
	var i = hp.lastIndexOf(':');
	if (i > 0 && /^\d+$/.test(hp.slice(i + 1)))
		return { host: hp.slice(0, i), port: hp.slice(i + 1) };
	return { host: hp, port: '' };
}

// Pulls what a person needs to tell links apart. Secrets (user ids, keys,
// passwords) are never part of the result.
function parseLink(raw) {
	var s = String(raw || '').trim();
	var out = { scheme: '', proto: '?', host: '', port: '', name: '', params: {} };
	var m = s.match(/^([a-z0-9+.-]+):\/\/(.*)$/i);
	if (!m)
		return out;
	out.scheme = m[1].toLowerCase();
	out.proto = protoLabel(out.scheme);
	var rest = m[2];

	if (out.scheme === 'vmess') {
		try {
			var o = JSON.parse(b64decode(rest.replace(/[#?].*$/, '')));
			out.host = String(o.add || '');
			out.port = String(o.port || '');
			out.name = String(o.ps || '');
			if (o.net) out.params.type = String(o.net);
			if (o.tls) out.params.security = String(o.tls);
			if (o.sni) out.params.sni = String(o.sni);
			return out;
		} catch (e) {}
	}
	if (out.scheme === 'vpn')
		return out;

	var hash = rest.indexOf('#');
	if (hash >= 0) {
		out.name = safeDecode(rest.slice(hash + 1));
		rest = rest.slice(0, hash);
	}
	var q = rest.indexOf('?');
	if (q >= 0) {
		rest.slice(q + 1).split('&').forEach(function(kv) {
			if (!kv)
				return;
			var eq = kv.indexOf('=');
			var k = eq >= 0 ? kv.slice(0, eq) : kv;
			out.params[safeDecode(k).toLowerCase()] = eq >= 0 ? safeDecode(kv.slice(eq + 1)) : '';
		});
		rest = rest.slice(0, q);
	}
	var at = rest.lastIndexOf('@');
	var hp = at >= 0 ? rest.slice(at + 1) : rest;
	if (at < 0 && out.scheme === 'ss') {
		var dec = b64decode(rest.replace(/\/.*$/, ''));
		if (dec && dec.lastIndexOf('@') >= 0)
			hp = dec.slice(dec.lastIndexOf('@') + 1);
	}
	var h = splitHostPort(hp);
	out.host = h.host;
	out.port = h.port;
	return out;
}

function linkFacts(p) {
	var facts = [];
	var pr = p.params || {};
	if (pr.security && pr.security !== 'none')
		facts.push(pr.security === 'reality' ? 'REALITY' : pr.security.toUpperCase());
	if (pr.type && pr.type !== 'tcp')
		facts.push(pr.type === 'ws' ? 'WebSocket' : (pr.type === 'grpc' ? 'gRPC' : pr.type));
	if (pr.flow && /vision/.test(pr.flow))
		facts.push('Vision');
	if (pr.sni || pr.peer)
		facts.push('SNI ' + (pr.sni || pr.peer));
	return facts;
}

// Channel names by engine tag, from list_routes: the user's own name or the
// generated "protocol host" one. Same names as on the Services page.
function channelNamesFrom(res) {
	var d = unwrapData(res);
	var map = {};
	var sections = (d && d.sections) || [];
	sections.forEach(function(sec) {
		(sec.channels || []).forEach(function(c) {
			if (c.tag && c.name)
				map[c.tag] = c.name;
		});
	});
	return map;
}

// Human name of a status channel: own name, else protocol and server, never
// the raw link or the internal tag.
var _channelNames = {};

function setChannelNames(map) {
	_channelNames = map || {};
}

function channelTitle(ch, names) {
	if (!ch)
		return '';
	names = names || _channelNames;
	if (names && ch.name && names[ch.name])
		return names[ch.name];
	if (ch.type === 'urltest')
		return _('Самый быстрый из группы');
	if (ch.type === 'direct' || /-awg-out$/.test(ch.name || ''))
		return _('Основной VPN');
	var d = String(ch.display || ch.name || '').replace(/\s*\([^)]*-out\)$/, '');
	var p = parseLink(d);
	if (p.scheme)
		return p.proto + ' ' + p.host + (p.port ? ':' + p.port : '');
	var proto = protoLabel(ch.type);
	if (proto && d.indexOf('://') < 0)
		return proto + ' ' + d;
	return d.length > 42 ? d.slice(0, 40) + '…' : d;
}

// Splits a channel name into protocol badge and the rest, so cards do not
// show "Hysteria2 hysteria2 1.2.3.4" when the generated name starts with it.
var NAME_PROTOS = { hysteria2: 'Hysteria2', hy2: 'Hysteria2', awg: 'AmneziaWG', awg2: 'AmneziaWG', vless: 'VLESS', vmess: 'VMess', trojan: 'Trojan', ss: 'Shadowsocks', shadowsocks: 'Shadowsocks', socks: 'SOCKS', socks5: 'SOCKS', tuic: 'TUIC', wireguard: 'WireGuard', hysteria: 'Hysteria' };

function channelParts(ch, names) {
	var title = channelTitle(ch, names);
	var proto = (ch && ch.type && ch.type !== 'urltest' && ch.type !== 'direct') ? protoLabel(ch.type) : '';
	var m = String(title).match(/^(\S+)\s+(.+)$/);
	if (m && NAME_PROTOS[m[1].toLowerCase()]) {
		proto = NAME_PROTOS[m[1].toLowerCase()];
		title = m[2];
	}
	else if (m && proto && m[1].toLowerCase() === proto.toLowerCase())
		title = m[2];
	return { proto: proto, title: title };
}

function channelLabelNode(ch) {
	var parts = channelParts(ch);
	return E('span', { 'class': 'hf-chname' }, [
		parts.proto ? E('span', { 'class': 'hf-proto' }, parts.proto) : '',
		E('span', {}, parts.title)
	]);
}

function tagTitle(tag, data, names) {
	if (!tag)
		return '';
	names = names || _channelNames;
	var chans = (data && data.channels) || [];
	for (var i = 0; i < chans.length; i++)
		if (chans[i].name === tag)
			return channelTitle(chans[i], names);
	if (names && names[tag])
		return names[tag];
	if (/-urltest-out$/.test(tag))
		return _('Самый быстрый из группы');
	if (/-awg-out$/.test(tag))
		return _('Основной VPN');
	if (tag === 'direct')
		return _('Напрямую');
	return tag;
}

function policyName(policy) {
	switch (policy) {
	case 'outage-only':
		return _('Резерв только при падении VPN');
	case 'prefer-primary':
		return _('Предпочитать VPN');
	case 'fastest':
		return _('Самый быстрый канал');
	default:
		return policy || '-';
	}
}

function modeName(mode) {
	switch (mode) {
	case 'urltest':
		return _('выбор самого быстрого');
	case 'primary':
		return _('на основном VPN');
	case 'backup':
		return _('на резервном канале');
	default:
		return mode || '-';
	}
}

function pill(text, state, title) {
	return E('span', {
		'class': 'hf-pill' + (state ? ' hf-pill--' + state : ''),
		'title': title || null
	}, text);
}

// Page header used by every page: title, live state pills, actions, hint and
// a result box that page actions write into.
function pageHeader(opts) {
	opts = opts || {};
	var result = E('pre', { 'class': 'hf-result' });
	var node = E('div', { 'class': 'hf-head' }, [
		E('div', { 'class': 'hf-head__info' }, [
			E('h2', { 'class': 'hf-head__title' }, opts.title || ''),
			E('div', { 'class': 'hf-head__state' }, opts.pills || [])
		]),
		E('div', { 'class': 'hf-head__actions' }, opts.actions || []),
		opts.hint ? E('p', { 'class': 'hf-head__hint' }, opts.hint) : '',
		result
	]);
	node.setResult = function(text, state) {
		result.textContent = text || '';
		result.className = 'hf-result' + (text ? ' hf-result--' + (state === true ? 'ok' : state === false ? 'bad' : (state || 'info')) : '');
	};
	node.setPills = function(pills) {
		var el = node.querySelector('.hf-head__state');
		emptyNode(el);
		(pills || []).forEach(function(p) { el.appendChild(p); });
	};
	return node;
}

var _moreCloser = false;

function moreMenu(label, items) {
	var menu = E('div', { 'class': 'hf-more__menu' });
	var wrap = E('div', { 'class': 'hf-more' }, [
		E('button', {
			'class': 'btn cbi-button cbi-button-neutral',
			'click': function(ev) {
				ev.preventDefault();
				ev.stopPropagation();
				wrap.classList.toggle('hf-more--open');
			}
		}, label || _('Ещё ▾')),
		menu
	]);
	if (!_moreCloser) {
		_moreCloser = true;
		document.addEventListener('click', function() {
			document.querySelectorAll('.hf-more--open').forEach(function(el) { el.classList.remove('hf-more--open'); });
		});
	}
	items.forEach(function(it) {
		if (!it)
			return;
		menu.appendChild(E('button', {
			'class': it.danger ? 'hf-more__danger' : '',
			'click': ui.createHandlerFn(null, function(ev) {
				wrap.classList.remove('hf-more--open');
				return it.fn(ev);
			})
		}, it.label));
	});
	return wrap;
}

function panel(title, sub, children, opts) {
	opts = opts || {};
	return E('div', { 'class': 'hf-panel' + (opts.wide ? ' hf-panel--wide' : ''), 'id': opts.id || null }, [
		E('div', { 'class': 'hf-panel__head' }, [
			E('div', {}, [
				E('h3', { 'class': 'hf-panel__title' }, title),
				sub ? E('p', { 'class': 'hf-panel__sub' }, sub) : ''
			]),
			opts.aside || ''
		])
	].concat(children || []));
}

function stat(label, value, state, title) {
	return E('div', { 'class': 'hf-stat' + (state ? ' hf-stat--' + state : ''), 'title': title || null }, [
		E('span', { 'class': 'hf-stat__label' }, label),
		E('span', { 'class': 'hf-stat__value' }, value)
	]);
}

function kvList(rows) {
	var dl = E('dl', { 'class': 'hf-kv' });
	rows.forEach(function(r) {
		if (!r)
			return;
		dl.appendChild(E('dt', {}, r[0]));
		dl.appendChild(E('dd', {}, (r[1] != null && r[1].nodeType) ? [ r[1] ] : String(r[1] != null && r[1] !== '' ? r[1] : '-')));
	});
	return dl;
}

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
var rpcListRoutes = rpc.declare({ object: 'hybrid-failover', method: 'list_routes' });

// LuCI default XHR timeout is 20s; health/global-check can need more on slow links.
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

function rpcHealthLong() {
	return withRpcTimeout(60, function() { return rpcHealth(); });
}

function rpcGlobalCheckLong() {
	return withRpcTimeout(60, function() { return rpcGlobalCheck(); });
}

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
	var engineUp = proxyRunning(data);
	var critical = engineUp && data.nft_ok && controlOk(data);
	if (!critical)
		return 'down';
	var ctrl = controllerForSection(data, data && data.failover && data.failover.section);
	if (ctrl && primaryProbeApplicable(ctrl) && ctrl.mode === 'backup' && ctrl.primary_ok === false)
		return 'degraded';
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

/** Primary VPN probe applies only to outage-only / prefer-primary, not pure urltest. */
function primaryProbeApplicable(ctrl) {
	if (!ctrl)
		return false;
	if (ctrl.mode === 'urltest' || ctrl.policy === 'fastest')
		return false;
	return ctrl.mode === 'primary' || ctrl.mode === 'backup';
}

function primaryProbeBadge(ctrl) {
	if (!ctrl)
		return '-';
	if (!primaryProbeApplicable(ctrl))
		return '-';
	return badge(!!ctrl.primary_ok, _('отвечает'), _('не отвечает'));
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

function channelAliveState(ch, probed, ctrl, data) {
	if (data && !proxyRunning(data))
		return 'unknown';
	var kind = channelKind(ch);
	if (kind === 'primary' && ctrl && primaryProbeApplicable(ctrl)) {
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

function channelStatusBadge(ch, probed, ctrl, data) {
	var st = channelAliveState(ch, probed, ctrl, data);
	if (st === 'up')
		return badge(true, _('на связи'), '');
	if (st === 'down')
		return badge(false, '', _('нет связи'));
	return badgeWarn(_('не проверен'));
}

function channelIsActiveReserve(ch, ctrl) {
	if (!ch)
		return false;
	if (ch.selected)
		return true;
	if (!ctrl || ctrl.urltest_member !== ch.name)
		return false;
	var active = ctrl.active || '';
	if (!active || /-urltest-out$/.test(active) || active === ch.name)
		return true;
	return false;
}

function channelRoleLabel(ch, ctrl) {
	var kind = channelKind(ch);
	if (kind === 'primary')
		return _('Основной VPN');
	if (kind === 'urltest')
		return _('Группа');
	if (channelIsActiveReserve(ch, ctrl))
		return _('Сейчас через него');
	return _('Резерв');
}

function channelsReserveSummary(channels, data, probed) {
	if (!channels || !channels.length)
		return '';
	if (!proxyRunning(data))
		return _('движок остановлен');
	var ctrl = controllerForSection(data, data && data.failover && data.failover.section);
	var alive = 0, total = 0;
	for (var i = 0; i < channels.length; i++) {
		if (channelKind(channels[i]) !== 'reserve')
			continue;
		total++;
		if (channelAliveState(channels[i], probed, ctrl, data) === 'up')
			alive++;
	}
	if (!total)
		return '';
	return _('%d из %d на связи').format(alive, total);
}

function buildChannelOverviewCard(ch, probed, ctrl, data) {
	var kind = channelKind(ch);
	var alive = channelAliveState(ch, probed, ctrl, data);
	var cardCls = 'hf-ent-channel-card hf-ent-channel-card--' + alive;
	var activeRes = channelIsActiveReserve(ch, ctrl) && kind === 'reserve';
	if (activeRes)
		cardCls += ' hf-ent-channel-card--active';
	var metaParts = [];
	if (ch.delay_ms && ch.delay_ms > 0)
		metaParts.push(ch.delay_ms + ' ' + _('мс'));
	else if (ch.detail && alive === 'up')
		metaParts.push(_('соединение установлено'));
	var body = [
		E('div', { 'class': 'hf-ent-channel-card__head' }, [
			E('span', { 'class': 'hf-ent-channel-card__role' }, channelRoleLabel(ch, ctrl)),
			channelStatusBadge(ch, probed, ctrl, data)
		]),
		E('div', { 'class': 'hf-ent-channel-card__name' }, channelLabelNode(ch)),
		E('div', { 'class': 'hf-ent-channel-card__meta' }, metaParts.join(' · ') || '-')
	];
	if (kind === 'primary' && ctrl && ctrl.last_error && ctrl.primary_ok === false)
		body.push(E('div', { 'class': 'hf-ent-channel-card__err' }, ctrl.last_error));
	else if (ch.detail && alive !== 'up')
		body.push(E('div', { 'class': 'hf-ent-channel-card__err', 'style': alive === 'unknown' ? 'color:var(--hf-warn);' : '' }, ch.detail));
	return E('div', { 'class': cardCls }, body);
}

function buildChannelsOverview(channels, data, probed, opts) {
	opts = opts || {};
	var ctrl = controllerForSection(data, opts.section);
	var head = E('div', { 'class': 'hf-ent-section-head' }, [
		E('h3', {}, _('Каналы')),
		typeof opts.onProbe === 'function' ? E('button', {
			'class': 'btn cbi-button cbi-button-action',
			'id': 'hf-btn-probe-overview',
			'click': opts.onProbe
		}, _('Проверить сейчас')) : ''
	]);
	if (!channels || !channels.length) {
		return E('div', { 'class': 'hf-ent-card hf-ent-channels' }, [
			head,
			E('p', { 'class': 'hf-mon-empty' },
				_('Каналов нет. Добавьте ссылки на странице «Маршрутизация».'))
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
		summaryParts.push(reserveSummary);
	if (ctrl && ctrl.mode)
		summaryParts.push(_('режим: ') + modeName(ctrl.mode));

	var grid = E('div', { 'class': 'hf-ent-channel-grid' });
	for (var p = 0; p < primary.length; p++)
		grid.appendChild(buildChannelOverviewCard(primary[p], probed, ctrl, data));
	for (var r = 0; r < reserves.length; r++)
		grid.appendChild(buildChannelOverviewCard(reserves[r], probed, ctrl, data));

	return E('div', { 'class': 'hf-ent-card hf-ent-channels' }, [
		head,
		summaryParts.length ? E('p', { 'class': 'hf-ent-channels__summary' }, summaryParts.join(' · ')) : '',
		grid,
		!probed ? E('p', { 'class': 'hint', 'style': 'margin:10px 0 0;font-size:12px;' },
			_('Задержки по последней фоновой проверке. «Проверить сейчас» проверит все каналы заново.')) : ''
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
		return _('Трафик идёт через VPN, пока он отвечает. Если VPN упал, включается резервный канал.');
	case 'prefer-primary':
		return _('Как только VPN снова отвечает, трафик сразу возвращается на него.');
	case 'fastest':
		return _('Трафик всегда идёт через самый быстрый из живых каналов.');
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
	switch (state) {
	case 'up':
		return pill(_('Всё работает'), 'ok');
	case 'degraded':
		return pill(_('Есть предупреждения'), 'warn');
	case 'down':
		return pill(_('Не работает'), 'bad');
	default:
		return pill(_('Нет данных'), '');
	}
}

function buildEnterpriseMeta(data) {
	var m = data && data.meta;
	if (!m || !m.core_version)
		return '';
	return pill('v' + m.core_version, 'plain');
}

function buildStatusHero(data, opts) {
	opts = opts || {};
	var state = overallState(data);
	var heroCls = 'hf-ent-hero hf-ent-hero--' +
		(state === 'up' ? 'ok' : state === 'degraded' ? 'warn' : state === 'down' ? 'bad' : 'neutral');
	var title, sub;
	if (state === 'up') {
		title = _('Трафик идёт через туннель');
		sub = _('Движок, перехват трафика и DNS работают.');
	} else if (state === 'degraded') {
		title = _('Работает с предупреждениями');
		sub = _('Трафик идёт, но что-то требует внимания. Подробности ниже.');
	} else if (state === 'down') {
		title = _('Туннель не работает');
		sub = _('Движок или перехват трафика не отвечают. Запустите полную проверку на странице «Диагностика».');
	} else {
		title = _('Нет данных');
		sub = _('Не удалось получить состояние с роутера.');
	}
	var main = E('div', { 'class': 'hf-ent-hero__main' }, [
		E('p', { 'class': 'hf-ent-hero__title' }, title),
		E('p', { 'class': 'hf-ent-hero__sub' }, sub)
	]);
	var errs = visibleErrors(data);
	if (errs.length)
		main.appendChild(E('div', { 'class': 'hf-ent-hero__sub', 'style': 'margin-top:8px;color:var(--hf-bad);' }, errs.join(' · ')));
	if (opts.primaryError)
		main.appendChild(E('div', { 'class': 'hf-ent-hero__sub', 'style': 'margin-top:6px;color:var(--hf-bad);' },
			_('Основной VPN') + ': ' + opts.primaryError));
	return E('div', { 'class': heroCls }, [main]);
}

// The channel traffic really goes through: the member a URLTest group picked,
// not the group itself.
function activeChannelTag(data, section) {
	var fo = data && data.failover;
	var ctrl = controllerForSection(data, section);
	var tag = activeOutboundTag(data, section);
	if (/-urltest-out$/.test(tag || '')) {
		if (ctrl && ctrl.urltest_member)
			return ctrl.urltest_member;
		if (fo && fo.urltest_now)
			return fo.urltest_now;
	}
	return tag;
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
	var dnsState = '', dnsVal = _('не проверен');
	if (data) {
		if (data.fakeip_skipped)
			dnsVal = _('пропущен');
		else if (data.fakeip_ok != null) {
			dnsState = data.fakeip_ok ? 'ok' : 'bad';
			dnsVal = data.fakeip_ok ? _('в порядке') : _('ошибка');
		}
	}
	var ctrl = controllerForSection(data, section);
	var policy = (data && data.failover && data.failover.policy) || (ctrl && ctrl.policy) || '';
	var engineRunning = proxyRunning(data);
	var tag = activeChannelTag(data, section);
	var activeState = tag ? 'ok' : '';
	if (ctrl && primaryProbeApplicable(ctrl) && ctrl.mode === 'backup' && !ctrl.primary_ok)
		activeState = 'warn';
	var ch = (data && data.channels) || [];
	var alive = 0, total = 0;
	for (var i = 0; i < ch.length; i++) {
		if (channelKind(ch[i]) !== 'reserve')
			continue;
		total++;
		if (channelAliveState(ch[i], false, ctrl, data) === 'up')
			alive++;
	}
	var stats = [
		stat(_('Движок'), engineRunning ? _('работает') : _('остановлен'), engineRunning ? 'ok' : 'bad'),
		stat(_('Перехват трафика'), data && data.nft_ok ? _('в порядке') : _('ошибка'), data && data.nft_ok ? 'ok' : 'bad', _('Правила nftables, которые заворачивают трафик в движок')),
		stat(_('DNS (fake-IP)'), dnsVal, dnsState),
		stat(_('Сейчас через'), tag ? tagTitle(tag, data) : '-', activeState),
		stat(_('Каналы на связи'), total ? (alive + ' / ' + total) : '-', !total ? '' : (alive === total ? 'ok' : (alive ? 'warn' : 'bad'))),
		stat(_('Режим'), policyName(policy), '', policyHint(policy))
	];
	if (!isNativeEngine(data))
		stats.splice(2, 0, stat('Clash API', controlOk(data) ? _('в порядке') : _('недоступен'), controlOk(data) ? 'ok' : 'bad'));
	return E('div', { 'class': 'hf-stats' }, stats);
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
		E('th', {}, _('Политика')),
		E('th', {}, _('Состояние')),
		E('th', {}, _('Активный тег')),
		E('th', {}, _('Основной VPN')),
		E('th', {}, _('Проверка')),
		E('th', {}, _('Счётчики'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < controllers.length; i++) {
		var c = controllers[i];
		if (sectionFilter && c.section !== sectionFilter)
			continue;
		var probeInfo = [];
		if (c.last_probe_at)
			probeInfo.push(formatRelativeTime(c.last_probe_at));
		if (c.last_error)
			probeInfo.push(c.last_error);
		tbody.appendChild(E('tr', {}, [
			E('td', {}, c.section || '-'),
			E('td', {}, c.policy || '-'),
			E('td', {}, c.mode || '-'),
			E('td', {}, E('span', { 'class': 'hf-mon-tag' }, c.active || '-')),
			E('td', {}, primaryProbeBadge(c)),
			E('td', { 'title': c.last_error || '' }, probeInfo.join(' · ') || '-'),
			E('td', {}, streakChip(c.fail_streak, 2, _('сбоев')) + ' · ' + streakChip(c.recover_streak, 2, _('успехов')))
		]));
	}
	var hintEl = '';
	if (dryRun && dryRun.length) {
		var ul = E('ul', { 'style': 'margin:8px 0 0;padding-left:18px;font-size:12px;' });
		for (var h = 0; h < dryRun.length; h++) {
			if (sectionFilter && dryRun[h].section !== sectionFilter)
				continue;
			ul.appendChild(E('li', {}, [ E('strong', {}, dryRun[h].section + ': '), dryRun[h].suggestion ]));
		}
		hintEl = ul;
	}
	return E('details', { 'class': 'hf-details' }, [
		E('summary', {}, _('Технические подробности')),
		wrapTable(E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody])),
		hintEl
	]);
}

function buildFailoverPanels(data, sectionFilter) {
	var fo = data && data.failover;
	var ctrl = null;
	if (data && data.controller) {
		for (var ci = 0; ci < data.controller.length; ci++) {
			var c = data.controller[ci];
			if ((sectionFilter && c.section === sectionFilter) || (!sectionFilter && fo && c.section === fo.section)) {
				ctrl = c;
				break;
			}
		}
		if (!ctrl && data.controller.length)
			ctrl = data.controller[0];
	}
	var policy = (ctrl && ctrl.policy) || (fo && fo.policy) || '';
	var tag = activeChannelTag(data, sectionFilter);
	var routeRows = [
		[_('Секция'), (ctrl && ctrl.section) || (fo && fo.section) || sectionFilter || '-'],
		[_('Режим'), policyName(policy)],
		[_('Как работает'), policyHint(policy)],
		[_('Сейчас через'), tag ? tagTitle(tag, data) : '-'],
		[_('На этом канале'), ctrl && ctrl.active_since ? formatRelativeTime(ctrl.active_since) : '-'],
		[_('Последнее переключение'), ctrl && ctrl.last_switch_at ? formatRelativeTime(ctrl.last_switch_at) : _('не было')]
	];
	var checkRows = [];
	if (primaryProbeApplicable(ctrl)) {
		checkRows.push([_('Основной VPN'), primaryProbeBadge(ctrl)]);
		if (ctrl.primary_delay_ms)
			checkRows.push([_('Задержка VPN'), ctrl.primary_delay_ms + ' ' + _('мс')]);
		if (ctrl.last_probe_at)
			checkRows.push([_('Проверен'), formatRelativeTime(ctrl.last_probe_at)]);
		if (ctrl.last_error)
			checkRows.push([_('Ошибка'), E('span', { 'style': 'color:var(--hf-bad);' }, ctrl.last_error)]);
	}
	if (fo) {
		if (fo.check_interval)
			checkRows.push([_('Проверка каналов'), _('каждые ') + fo.check_interval]);
		if (fo.tolerance)
			checkRows.push([_('Порог переключения'), fo.tolerance + ' ' + _('мс')]);
		if (fo.idle_timeout)
			checkRows.push([_('Пауза без трафика'), fo.idle_timeout]);
		if (fo.testing_url)
			checkRows.push([_('Адрес проверки'), E('span', { 'class': 'hf-mon-tag' }, fo.testing_url)]);
	}
	return E('div', {}, [
		E('div', { 'class': 'hf-panels', 'style': 'margin-top:16px;' }, [
			panel(_('Маршрут'), '', [ kvList(routeRows) ]),
			checkRows.length ? panel(_('Проверки'), '', [ kvList(checkRows) ]) : ''
		]),
		buildControllerTable(data && data.controller, sectionFilter, data && data.dry_run)
	]);
}

function buildChannelsTable(channels, probed, serverDelayData, nativeEngine, data) {
	if (!channels || !channels.length)
		return E('p', { 'class': 'hf-mon-empty' }, _('Каналов нет. Добавьте ссылки на странице «Маршрутизация».'));

	var ctrl = controllerForSection(data, data && data.failover && data.failover.section);
	var maxMs = maxChannelDelay(channels);
	var thead = E('tr', {}, [
		E('th', {}, _('Состояние')),
		E('th', {}, _('Канал')),
		E('th', {}, _('Задержка')),
		E('th', {}, _('Динамика')),
		E('th', {}, _('Роль'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < channels.length; i++) {
		var ch = channels[i];
		var statusCell = channelStatusBadge(ch, probed, ctrl, data);
		if (ch.detail && channelAliveState(ch, probed, ctrl, data) !== 'up')
			statusCell = E('div', {}, [ statusCell, E('div', { 'style': 'font-size:11px;opacity:.75;margin-top:2px;' }, ch.detail) ]);
		tbody.appendChild(E('tr', { 'class': ch.selected ? 'hf-mon-row--active' : '' }, [
			E('td', {}, statusCell),
			E('td', {}, channelLabelNode(ch)),
			E('td', {}, latencyBar(ch.delay_ms || 0, maxMs)),
			E('td', {}, buildSparklineSVG(ch.name, serverDelayData)),
			E('td', {}, channelRoleLabel(ch, ctrl))
		]));
	}
	return E('div', {}, [
		wrapTable(E('table', { 'class': 'hf-mon-table' }, [E('thead', {}, [thead]), tbody])),
		!probed ? E('p', { 'class': 'hint', 'style': 'font-size:12px;margin:6px 0 0;' },
			_('Задержки по последней фоновой проверке. «Проверить сейчас» проверит все каналы заново.')) : ''
	]);
}

var SWITCH_REASONS = {
	'manual': _('вручную'),
	'restore backup': _('возврат на резерв после перезапуска'),
	'primary outage': _('основной VPN перестал отвечать'),
	'primary recovered': _('основной VPN снова отвечает'),
	'backup urltest': _('выбор самого быстрого резерва'),
	'sync primary': _('синхронизация с основным VPN'),
	'sync backup': _('синхронизация с резервом'),
	'urltest failover': _('канал перестал отвечать')
};

function switchReason(r) {
	return SWITCH_REASONS[String(r || '')] || r || '-';
}

function buildHistoryTable(events, sectionFilter, limit, data) {
	if (!events || !events.length)
		return E('div', { 'class': 'hf-empty' }, _('Переключений пока не было.'));

	var list = events.slice().reverse();
	if (sectionFilter)
		list = list.filter(function(ev) { return (ev.section || ev.Section) === sectionFilter; });
	limit = limit || 15;
	if (list.length > limit)
		list = list.slice(0, limit);

	var thead = E('tr', {}, [
		E('th', {}, _('Время')),
		E('th', {}, _('Откуда')),
		E('th', {}, _('Куда')),
		E('th', {}, _('Причина')),
		E('th', {}, _('Задержка'))
	]);
	var tbody = E('tbody');
	for (var i = 0; i < list.length; i++) {
		var ev = list[i];
		var from = ev.from || ev.From || '';
		var to = ev.to || ev.To || '';
		tbody.appendChild(E('tr', {}, [
			E('td', { 'title': formatEventTime(ev.time || ev.Time) }, formatRelativeTime(ev.time || ev.Time)),
			E('td', { 'title': from }, from ? tagTitle(from, data) : '-'),
			E('td', { 'title': to }, to ? tagTitle(to, data) : '-'),
			E('td', {}, switchReason(ev.reason || ev.Reason)),
			E('td', {}, ev.probe_ms ? (ev.probe_ms + ' ' + _('мс')) : '-')
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
			}, onConfirm ? _('Отмена') : _('Закрыть')),
			!onConfirm ? '' : E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': function() {
					// A rejected promise from onConfirm keeps the dialog open
					// (form validation); anything else closes it.
					var close = function() {
						if (backdrop.parentNode)
							document.body.removeChild(backdrop);
					};
					var res = onConfirm ? onConfirm() : null;
					if (res && typeof res.then === 'function')
						res.then(close, function(err) {
							if (err)
								close();
						});
					else
						close();
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
		try {
			d = JSON.parse(d);
		} catch (e) {
			d.split('\n').forEach(function(line) {
				line = line.trim();
				if (line)
					items.push({ ok: !/^fail|error/i.test(line), text: line });
			});
			return items;
		}
	}
	if (d && d.report && typeof d.report === 'object') {
		d = Object.assign({}, d.report, {
			ok: d.ok,
			message: d.message,
			errors: d.errors || d.report.errors
		});
	}
	if (d.ok === true || d.ok === false)
		items.push({ ok: !!d.ok, text: d.message || (d.ok ? _('Проверка пройдена') : _('Проверка не пройдена')) });
	if (d.engine_running != null || d.singbox_running != null) {
		var run = d.engine_running != null ? d.engine_running : d.singbox_running;
		items.push({ ok: !!run, text: run ? _('Движок работает') : _('Движок остановлен') });
	}
	if (d.nft_ok != null)
		items.push({ ok: !!d.nft_ok, text: d.nft_ok ? _('Правила перехвата трафика (nftables) на месте') : _('Правила перехвата трафика (nftables) не найдены') });
	if (d.fakeip_ok != null)
		items.push({ ok: !!d.fakeip_ok, text: d.fakeip_ok ? _('DNS отдаёт fake-IP, домены из списков пойдут в туннель') : _('DNS не отдаёт fake-IP, домены из списков могут идти мимо туннеля') });
	if (!isNativeEngine(d) && d.clash_ok != null)
		items.push({ ok: !!d.clash_ok, text: 'Clash API: ' + (d.clash_ok ? _('в порядке') : _('недоступен')) });
	if (d.active_outbound)
		items.push({ ok: true, text: _('Активный канал: ') + tagTitle(d.active_outbound, d) });
	if (d.errors && Array.isArray(d.errors))
		d.errors.forEach(function(e) { items.push({ ok: false, text: String(e) }); });
	return items;
}

function renderChecklist(items) {
	if (!items.length)
		return E('div', { 'class': 'hf-checks__empty' }, '-');
	return E('ul', { 'class': 'hf-checks' }, items.map(function(it) {
		return E('li', { 'class': it.ok ? '' : 'hf-checks--bad' }, [
			E('span', { 'class': 'hf-checks__mark' }, it.ok ? '✓' : '!'),
			E('span', {}, it.text)
		]);
	}));
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
	parent.appendChild(E('style', { 'type': 'text/css' }, HF_CSS + '\n' + HF_PAGE_CSS));
}

return baseclass.extend({
	HF_CSS: HF_CSS,
	HF_DELAY_MAX: HF_DELAY_MAX,
	rpc: {
		status: rpcStatus,
		health: rpcHealthLong,
		history: rpcHistory,
		delayHistory: rpcDelayHistory,
		checkFakeip: rpcCheckFakeip,
		exportHistory: rpcExportHistory,
		switchProxy: rpcSwitchProxy,
		globalCheck: rpcGlobalCheckLong,
		listClients: rpcListClients,
		dhcpLeases: rpcDhcpLeases,
		metrics: rpcMetrics,
		listRoutes: rpcListRoutes
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
	injectStyles: injectStyles,
	protoLabel: protoLabel,
	parseLink: parseLink,
	linkFacts: linkFacts,
	channelNamesFrom: channelNamesFrom,
	channelTitle: channelTitle,
	channelParts: channelParts,
	channelLabelNode: channelLabelNode,
	setChannelNames: setChannelNames,
	activeChannelTag: activeChannelTag,
	tagTitle: tagTitle,
	policyName: policyName,
	modeName: modeName,
	pill: pill,
	pageHeader: pageHeader,
	moreMenu: moreMenu,
	panel: panel,
	stat: stat,
	kvList: kvList
});
