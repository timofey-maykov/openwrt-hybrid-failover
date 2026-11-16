const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const source = fs.readFileSync('luci/root/www/luci-static/resources/hybrid-failover/hf-ui.js', 'utf8');
function E(tag, attrs, children) { return {tag, attrs, children, appendChild(n) { this.children.push(n); }}; }
const ctx = { E, _: s => s, proxyRunning: d => !!d.engine_running, controlOk: () => true, visibleErrors: () => [], primaryProbeApplicable: () => false };
vm.createContext(ctx);
for (const [start,end] of [['function controllerForSection(', 'function activeOutboundDisplay('],['function sectionDisabled(', 'function badge('],['function buildStatusPill(', 'function buildEnterpriseMeta('],['function buildStatusHero(', '// The channel traffic really']])
 vm.runInContext(source.slice(source.indexOf(start),source.indexOf(end)),ctx);
const data = {section_disabled:true,engine_running:true,nft_ok:true,failover:{section:'glob',selector_now:'glob-1-out'},controller:[{section:'glob',active:'glob-1-out'}],channels:[{name:'glob-1-out',available:false}]};
assert.equal(ctx.overallState(data),'disabled');
assert.equal(ctx.activeOutboundTag(data),'');
assert.equal(ctx.sectionDisabled(data,'other'),false);
let hero = JSON.stringify(ctx.buildStatusHero(data,{}));
assert.ok(hero.includes('Секция выключена'));
assert.ok(!hero.includes('Трафик идёт через туннель'));
data.section_disabled=false;
assert.equal(ctx.overallState(data),'up');
assert.equal(ctx.activeOutboundTag(data),'glob-1-out');
hero=JSON.stringify(ctx.buildStatusHero(data,{}));
assert.ok(hero.includes('Движок и маршрутизация работают'));
const dashboard=fs.readFileSync('luci/root/www/luci-static/resources/view/hybrid-failover/dashboard.js','utf8');
const cacheCtx={hfui:{sectionDisabled:d=>!!d.section_disabled,recordDelayHistoryLocal(){},setChannelNames(){}}};
vm.createContext(cacheCtx);
const view=vm.runInContext('({'+dashboard.slice(dashboard.indexOf('\tapplyLoadResults:'),dashboard.indexOf('\theaderPills:'))+'})',cacheCtx);
view._channelData=[{name:'stale'}];view._channelsProbed=true;
view.applyLoadResults([{ok:true,data:{section_disabled:true}},null,null,null]);
assert.equal(view._channelData,null);assert.equal(view._channelsProbed,false);
console.log('disabled section: neutral status, no active channel, cleared probe cache; enabled status preserved');
