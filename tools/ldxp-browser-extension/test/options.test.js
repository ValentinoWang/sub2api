import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
const requireFrontend = createRequire(new URL('../../../frontend/package.json', import.meta.url));
const { JSDOM } = requireFrontend('jsdom');
const local = 'http://127.0.0.1:8080';
const production = 'https://ai.rest2build.lol';
const tick = () => new Promise(resolve => setImmediate(resolve));
async function harness() {
  const dom = new JSDOM(await readFile(new URL('../options.html', import.meta.url), 'utf8'));
  const previous = { document: globalThis.document, chrome: globalThis.chrome };
  globalThis.document = dom.window.document;
  const calls = [];
  const data = { busy: false, bindings: [], sites: [{ site: local, has_credential: true }, { site: production, has_credential: true }] };
  const h = { failSave: false, failInspect: '', data, calls, dom, document: dom.window.document };
  globalThis.chrome = {
    storage: { onChanged: { addListener: listener => { h.onChanged = listener; } } },
    permissions: { request: async () => true },
    runtime: { sendMessage: async message => {
      calls.push(structuredClone(message));
      if (message.type === 'snapshot') return { ok: true, data };
      if (message.type === 'inspect') {
        if (message.binding.site === h.failInspect) return { ok: false, message: '凭据无效' };
        const offset = message.binding.site === local ? 100 : 200;
        return { ok: true, data: { device_id: `${offset}`, products: [5, 10, 20].map((amount, index) => ({ goods_id: offset + index, title: `${amount}元额度`, cny_amount: amount, usd_credit: amount })) } };
      }
      if (message.type === 'maintain') {
        if (h.failSave) return { ok: false, message: '站点核对失败' };
        return { ok: true, data: { ...data, maintenance: { requested: message.sites.reduce((sum, site) => sum + site.goods_ids.length, 0), running: message.sites.reduce((sum, site) => sum + site.goods_ids.length, 0), issues: [] } } };
      }
      throw new Error('unexpected message');
    } },
  };
  await import(`../options.js?test=${Math.random()}`);
  h.card = site => h.document.querySelector(`[data-site="${site}"]`);
  h.read = async () => { h.document.querySelector('#inspect-all').click(); await tick(); };
  h.check = (site, indices) => {
    const inputs = h.card(site).querySelectorAll('input[type=checkbox]');
    for (const [index, input] of inputs.entries()) { input.checked = indices.includes(index); input.dispatchEvent(new dom.window.Event('change')); }
  };
  h.save = async () => { h.document.querySelector('button[type=submit]').click(); await tick(); };
  h.cleanup = () => { globalThis.document = previous.document; globalThis.chrome = previous.chrome; dom.window.close(); };
  return h;
}
test('two permanent site panels reuse saved credentials without inserting secrets into fields', async () => {
  const h = await harness();
  try {
    assert.equal(h.document.querySelectorAll('.site-card').length, 2);
    for (const site of [local, production]) {
      assert.match(h.card(site).textContent, /凭据已保存/);
      assert.equal(h.card(site).querySelector('input[type=password]').value, '');
    }
    await h.read();
    assert.equal(h.document.querySelectorAll('input[type=checkbox]').length, 6);
    assert.equal(h.document.querySelectorAll('input:checked').length, 6);
    assert.equal(h.document.querySelector('#selection-summary').textContent, '已选 2 个站点 · 6 个额度');
    assert.equal(h.document.querySelector('button[type=submit]').textContent, '保存并自动补货');
    for (const call of h.calls.filter(call => call.type === 'inspect')) assert.equal(call.binding.device_key, '');
  } finally { h.cleanup(); }
});
test('one submit sends both sites and every checked amount, then leaves controls usable', async () => {
  const h = await harness();
  try {
    await h.read(); h.check(local, [0, 1]); h.check(production, [0, 2]);
    assert.equal(h.document.querySelector('#selection-summary').textContent, '已选 2 个站点 · 4 个额度');
    const submit = h.document.querySelector('button[type=submit]');
    assert.equal(submit.disabled, false);
    await h.save();
    const calls = h.calls.filter(call => call.type === 'maintain');
    assert.equal(calls.length, 1);
    assert.equal(calls[0].target_stock, 999);
    assert.deepEqual(calls[0].sites.map(site => site.goods_ids), [[100, 101], [200, 202]]);
    assert.match(h.document.querySelector('#feedback').textContent, /4 个商品已配对，4 个已开启自动补货/);
    h.check(local, [0]);
    assert.equal(submit.disabled, false);
  } finally { h.cleanup(); }
});
test('editing a credential invalidates only that site, while the other site remains selectable', async () => {
  const h = await harness();
  try {
    await h.read(); h.check(local, [0]); h.check(production, [1]);
    const key = h.card(local).querySelector('input[type=password]');
    key.value = 'changed'; key.dispatchEvent(new h.dom.window.Event('input'));
    assert.equal(h.card(local).querySelectorAll('input[type=checkbox]').length, 0);
    assert.equal(h.document.querySelector('#selection-summary').textContent, '已选 1 个站点 · 1 个额度');
    await h.save();
    assert.deepEqual(h.calls.find(call => call.type === 'maintain').sites.map(site => site.site), [production]);
  } finally { h.cleanup(); }
});
test('failed bulk save keeps selections and entered credentials ready for correction', async () => {
  const h = await harness();
  try {
    const key = h.card(local).querySelector('input[type=password]'); key.value = 'fixture-entered-key';
    await h.read(); h.check(local, [0, 1]); h.check(production, []); h.failSave = true;
    await h.save();
    assert.equal(key.value, 'fixture-entered-key');
    assert.equal(h.card(local).querySelectorAll('input:checked').length, 2);
    assert.equal(h.document.querySelector('button[type=submit]').disabled, false);
    assert.match(h.document.querySelector('#feedback').textContent, /操作未完成/);
  } finally { h.cleanup(); }
});
test('select all is per site and empty selections cannot save', async () => {
  const h = await harness();
  try {
    assert.equal(h.document.querySelector('button[type=submit]').disabled, true);
    await h.read();
    h.card(local).querySelector('.text-button').click();
    assert.equal(h.card(local).querySelectorAll('input:checked').length, 0);
    assert.equal(h.card(production).querySelectorAll('input:checked').length, 3);
    h.card(production).querySelector('.text-button').click();
    assert.equal(h.document.querySelector('button[type=submit]').disabled, true);
  } finally { h.cleanup(); }
});
test('a failed site read is visible and does not erase the other site products', async () => {
  const h = await harness();
  try {
    h.failInspect = production;
    await h.read();
    assert.equal(h.card(local).querySelectorAll('input[type=checkbox]').length, 3);
    assert.equal(h.card(production).querySelectorAll('input[type=checkbox]').length, 0);
    assert.match(h.card(production).querySelector('.site-status').textContent, /凭据无效/);
  } finally { h.cleanup(); }
});

test('an external worker cycle finishing releases the setup controls', async () => {
  const h = await harness();
  try {
    await h.read();
    h.data.busy = true;
    h.onChanged({ uiRevision: { newValue: 1 } }, 'local'); await tick();
    assert.equal(h.document.querySelector('button[type=submit]').disabled, true);
    h.data.busy = false;
    h.onChanged({ uiRevision: { newValue: 2 } }, 'local'); await tick();
    assert.equal(h.document.querySelector('button[type=submit]').disabled, false);
    assert.equal(h.document.querySelectorAll('input:checked').length, 6);
  } finally { h.cleanup(); }
});
