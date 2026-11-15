const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const source = fs.readFileSync('luci/root/www/luci-static/resources/view/hybrid-failover/routing.js', 'utf8');
const context = {
 L: { toArray: v => Array.isArray(v) ? v : v == null ? [] : [v] },
 _: s => s, Event,
 dom: { bindClassInstance() {} },
 ui: { AbstractElement: { extend(methods) {
  function Widget(...args) { methods.__init__.apply(this, args); }
  Widget.prototype = { setUpdateEvents() {}, setChangeEvents() {}, ...methods };
  return Widget;
 } } },
 E(tag, attrs, children) {
  const node = new EventTarget();
  node.attrs = { ...attrs }; node.children = children;
  node.setAttribute = (k, v) => { node.attrs[k] = v; };
  node.classList = { toggle(k, on) { node.attrs[k] = on; } };
  return node;
 }
};
vm.createContext(context);
vm.runInContext(source.slice(source.indexOf('var ChipsWidget ='), source.indexOf('var CBIChips =')), context);
const stored = ['youtube'];
const widget = new context.ChipsWidget(stored, { youtube: 'YouTube', telegram: 'Telegram' }, {});
widget.render();
const click = key => widget.chips[key].attrs.click({ preventDefault() {} });
click('telegram');
assert.deepEqual(stored, ['youtube'], 'click must not mutate LuCI cfgvalue or saving skips the write');
assert.deepEqual(Array.from(widget.getValue()), ['youtube', 'telegram']);
const saved = widget.getValue();
const reopened = new context.ChipsWidget(saved, { youtube: 'YouTube', telegram: 'Telegram' }, {});
reopened.render();
assert.equal(reopened.chips.telegram.attrs['aria-pressed'], 'true');
click('youtube'); click('telegram');
assert.equal(widget.getValue().length, 0, 'removing all lists must save an empty selection');
const reset = ['telegram'];
widget.setValue(reset);
assert.equal(widget.chips.youtube.attrs['aria-pressed'], 'false');
assert.equal(widget.chips.telegram.attrs['aria-pressed'], 'true');
click('telegram');
assert.deepEqual(reset, ['telegram'], 'setValue must also detach the saved array');
console.log('routing widgets: selection, clearing, reopening and setValue passed');
