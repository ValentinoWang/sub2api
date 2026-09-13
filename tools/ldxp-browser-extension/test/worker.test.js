import test from 'node:test';
import assert from 'node:assert/strict';
import { bindingID, initialState } from '../core.js';

async function harness(statePatch = {}) {
  const binding = { site: 'https://ai.rest2build.lol', config_id: 'ldxp', device_id: 'device1', goods_id: 42, name: '5元额度', device_key: `ldxpd_${'f'.repeat(64)}` };
  const id = bindingID(binding);
  const data = { bindings: { [id]: binding }, states: { [id]: { ...initialState(), ...statePatch } } };
  const listeners = {}; const access = []; const alarms = [];
  const previousChrome = globalThis.chrome;
  const previousFetch = globalThis.fetch;
  const calls = [];
  const configs = new Map();
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    const config = configs.get(new URL(url).origin);
    return { ok: !!config, json: async () => ({ code: 0, data: config }) };
  };
  globalThis.chrome = {
    storage: { local: {
      setAccessLevel: async value => access.push(value),
      get: async () => structuredClone(data),
      set: async value => Object.assign(data, structuredClone(value)),
    } },
    alarms: { create: async (...args) => alarms.push(args), onAlarm: { addListener: listener => { listeners.alarm = listener; } } },
    runtime: { id: 'self', getURL: page => `chrome-extension://self/${page}`, onMessage: { addListener: listener => { listeners.message = listener; } }, onStartup: { addListener: listener => { listeners.startup = listener; } } },
    action: { onClicked: { addListener: listener => { listeners.open = listener; } } },
    tabs: { query: async () => [] },
    permissions: { contains: async () => true },
  };
  await import(`../worker.js?test=${Math.random()}`);
  const sender = { id: 'self', url: 'chrome-extension://self/options.html', origin: 'chrome-extension://self', tab: { url: 'chrome-extension://self/options.html' } };
  const message = payload => new Promise(resolve => listeners.message(payload, sender, resolve));
  return { id, binding, data, calls, configs, access, alarms, listeners, message, cleanup: () => { globalThis.chrome = previousChrome; globalThis.fetch = previousFetch; } };
}
test('worker restart retains original uncertain batch and requires manual inventory check', async () => {
  const h = await harness({ enabled: true, phase: 'uploading', checked: true, batch: { batch_id: 'original', status: 'uncertain' } });
  try {
    const result = await h.message({ type: 'snapshot' });
    const item = result.data.bindings[0];
    assert.equal(item.enabled, false); assert.equal(item.checked, false); assert.equal(item.reason, 'uncertain'); assert.equal(item.pending, true);
    assert.equal(h.data.states[h.id].batch.batch_id, 'original');
    assert.equal(JSON.stringify(result).includes('ldxpd_'), false);
    assert.deepEqual(h.access, [{ accessLevel: 'TRUSTED_CONTEXTS' }]);
    assert.deepEqual(h.alarms, [['ldxp-stock-check', { periodInMinutes: 1 }]]);
  } finally { h.cleanup(); }
});
test('a web tab or external sender cannot issue inspect, start, or snapshot commands', async () => {
  const h = await harness();
  try {
    await h.message({ type: 'snapshot' });
    for (const sender of [
      { id: 'self', url: 'https://www.ldxp.cn/merchant/goods/list', origin: 'https://www.ldxp.cn' },
      { id: 'other', url: 'chrome-extension://self/popup.html' },
      { id: 'self', url: 'chrome-extension://self/popup.html', origin: 'https://evil.test' },
      { id: 'self', url: 'chrome-extension://self/options.html', tab: { url: 'https://evil.test' } },
    ]) assert.equal(h.listeners.message({ type: 'inspect' }, sender, () => assert.fail('must not reply to untrusted sender')), false);
  } finally { h.cleanup(); }
});
test('start requires a completed explicit check even with stored credentials', async () => {
  const h = await harness({ checked: false });
  try {
    const result = await h.message({ type: 'start', id: h.id });
    assert.equal(result.ok, false); assert.equal(result.error, 'protocol_error');
  } finally { h.cleanup(); }
});
test('binding with an unresolved batch cannot be removed', async () => {
  const h = await harness({ batch: { batch_id: 'pending', status: 'uncertain' } });
  try {
    const result = await h.message({ type: 'remove', id: h.id });
    assert.equal(result.ok, false); assert.equal(result.error, 'uncertain');
    assert.ok(h.data.bindings[h.id]);
  } finally { h.cleanup(); }
});

const local = 'http://127.0.0.1:8080';
const production = 'https://ai.rest2build.lol';
function catalog(device, goods) {
  return { enabled: false, device: { id: device }, products: goods.map(goods_id => ({ goods_id, title: `${goods_id}元额度`, enabled: true, cny_amount: 5, usd_credit: 5 })) };
}
function selections(h) {
  return [
    { site: local, device_id: 'local-device', device_key: `ldxpd_${'a'.repeat(64)}`, goods_ids: [101, 102] },
    { site: production, device_id: 'device1', goods_ids: [42, 43] },
  ];
}
test('two sites and multiple amounts bind atomically, preserve inventory and pending batches while reusing a saved credential', async () => {
  const h = await harness({ enabled: true, checked: true, stock: 12, lastSuccess: 123, batch: { batch_id: 'pending', status: 'uncertain' } });
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    h.configs.set(production, catalog('device1', [42, 43]));
    const result = await h.message({ type: 'maintain', target_stock: 999, sites: selections(h) });
    assert.equal(result.ok, true);
    assert.equal(result.data.bindings.length, 4);
    assert.equal(h.data.states[h.id].stock, 12);
    assert.equal(h.data.states[h.id].enabled, false);
    assert.equal(h.data.states[h.id].batch.batch_id, 'pending');
    for (const [id, state] of Object.entries(h.data.states)) if (id !== h.id) assert.equal(state.enabled, false);
    assert.equal(h.calls.length, 2);
    assert.equal(h.calls[0].options.headers.Authorization, `Bearer ldxpd_${'a'.repeat(64)}`);
    assert.equal(h.calls[1].options.headers.Authorization, `Bearer ${h.binding.device_key}`);
    assert.equal(JSON.stringify(result).includes('ldxpd_'), false);
    const before = structuredClone(h.data.states);
    const again = await h.message({ type: 'maintain', target_stock: 999, sites: selections(h).map(({ device_key, ...site }) => site) });
    assert.equal(again.ok, true);
    assert.equal(again.data.bindings.length, 4);
    assert.deepEqual(h.data.states, before);
  } finally { h.cleanup(); }
});
test('failure at the second site does not save the first site or credentials', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    h.configs.set(production, catalog('wrong-device', [42, 43]));
    const result = await h.message({ type: 'maintain', target_stock: 999, sites: selections(h) });
    assert.equal(result.ok, false);
    assert.equal(Object.keys(h.data.bindings).length, 1);
    assert.equal(h.data.siteCredentials, undefined);
  } finally { h.cleanup(); }
});
test('inspect reuses only the credential for that site and never returns it', async () => {
  const h = await harness();
  try {
    h.configs.set(production, catalog('device1', [42, 43]));
    const result = await h.message({ type: 'inspect', binding: { site: production } });
    assert.equal(result.ok, true);
    assert.equal(result.data.products.length, 2);
    assert.equal(JSON.stringify(result).includes('ldxpd_'), false);
    const wrong = await h.message({ type: 'inspect', binding: { site: local } });
    assert.equal(wrong.ok, false);
    assert.equal(h.calls.length, 1);
    const snapshot = await h.message({ type: 'snapshot' });
    assert.equal(snapshot.data.sites.find(site => site.site === production).has_credential, true);
    assert.equal(snapshot.data.sites.find(site => site.site === local).has_credential, false);
  } finally { h.cleanup(); }
});
test('one merchant product cannot be bound to two sites or two devices', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [42]));
    const result = await h.message({ type: 'maintain', target_stock: 999, sites: [{ ...selections(h)[0], goods_ids: [42] }] });
    assert.equal(result.ok, false);
    assert.equal(result.error, 'product_conflict');
    h.configs.set(production, catalog('new-device', [42]));
    const sameSite = await h.message({ type: 'maintain', target_stock: 999, sites: [{ site: production, device_id: 'new-device', device_key: `ldxpd_${'b'.repeat(64)}`, goods_ids: [42] }] });
    assert.equal(sameSite.ok, false);
    assert.equal(sameSite.error, 'product_conflict');
    assert.equal(Object.keys(h.data.bindings).length, 1);
  } finally { h.cleanup(); }
});
test('batch binding rejects empty, repeated and unauthorized selections without partial saves', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    for (const goods_ids of [[], [101, 101], [101, 999], [0], ['101']]) {
      const result = await h.message({ type: 'maintain', target_stock: 999, sites: [{ ...selections(h)[0], goods_ids }] });
      assert.equal(result.ok, false);
      assert.equal(Object.keys(h.data.bindings).length, 1);
    }
  } finally { h.cleanup(); }
});

async function enableMaintenanceFixture(h, { brokenSite = '', unavailableMerchant = false } = {}) {
  for (const config of h.configs.values()) config.enabled = true;
  const online = Object.getOwnPropertyDescriptor(navigator, 'onLine');
  Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
  const cleanup = h.cleanup;
  h.cleanup = () => { if (online) Object.defineProperty(navigator, 'onLine', online); else delete navigator.onLine; cleanup(); };
  chrome.tabs.query = async () => unavailableMerchant ? [] : [{ id: 9, url: 'https://www.ldxp.cn/merchant/goods/list', status: 'complete' }];
  chrome.scripting = { executeScript: async request => {
    assert.equal(request.args[0], 'inventory', 'this full-stock fixture must never upload');
    return [{ frameId: 0, result: { ok: true, inventory: { complete: true, total: 999, hashes: Array.from({ length: 999 }, (_, i) => (request.args[1] * 10000 + i).toString(16).padStart(64, '0')) } } }];
  } };
  globalThis.fetch = async (url, options) => {
    h.calls.push({ url, options });
    const site = new URL(url).origin;
    const config = h.configs.get(site);
    if (url.endsWith('/stock-target') && brokenSite === site) return { ok: false, status: 404 };
    let value;
    if (url.endsWith('/config')) value = config;
    else if (url.endsWith('/stock-target')) {
      const body = JSON.parse(options.body);
      value = { products: body.goods_ids.map(goods_id => ({ goods_id, target_stock: body.target_stock })) };
    } else if (url.endsWith('/heartbeat') || url.endsWith('/resume')) value = { enabled: true, paused_reason: '' };
    else if (url.endsWith('/inventory')) value = { identity_verified: true, blocked: false, batch_resolved: false, pending_batch: null, target_stock: 999 };
    else if (url.endsWith('/claim')) value = { batch: null };
    else assert.fail(`unexpected API ${new URL(url).pathname}`);
    return { ok: true, json: async () => ({ code: 0, data: value }) };
  };
}
test('one maintenance action binds both sites, sets 999 and automatically checks before starting', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    h.configs.set(production, catalog('device1', [42, 43]));
    await enableMaintenanceFixture(h);
    const result = await h.message({ type: 'maintain', sites: selections(h), target_stock: 999 });
    assert.equal(result.ok, true);
    assert.equal(result.data.maintenance.requested, 4);
    assert.equal(result.data.maintenance.running, 4);
    assert.equal(result.data.target_stock, 999);
    for (const item of result.data.bindings) { assert.equal(item.target_stock, 999); assert.equal(item.enabled, true); }
    const targets = h.calls.filter(call => call.url.endsWith('/stock-target'));
    assert.equal(targets.length, 2);
    assert.deepEqual(JSON.parse(targets[0].options.body), { goods_ids: [101, 102], target_stock: 999 });
    assert.equal(h.calls.filter(call => call.url.endsWith('/inventory')).length, 4);
    assert.equal(JSON.stringify(result).includes('ldxpd_'), false);
    const stopped = await h.message({ type: 'pause-all' });
    assert.equal(stopped.ok, true);
    assert.equal(stopped.data.bindings.some(item => item.enabled), false);
  } finally { h.cleanup(); }
});
test('an old backend leaves only that site paused and reports partial maintenance honestly', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    h.configs.set(production, catalog('device1', [42, 43]));
    await enableMaintenanceFixture(h, { brokenSite: production });
    const result = await h.message({ type: 'maintain', sites: selections(h), target_stock: 999 });
    assert.equal(result.ok, true);
    assert.equal(result.data.maintenance.running, 2);
    assert.equal(result.data.maintenance.issues[0].reason, 'upgrade_required');
    for (const binding of result.data.bindings.filter(item => item.site === production)) {
      assert.equal(binding.enabled, false);
      assert.notEqual(binding.target_stock, 999);
      assert.equal(binding.reason, 'upgrade_required');
    }
    assert.equal(h.calls.some(call => call.url.startsWith(production) && call.url.endsWith('/claim')), false);
  } finally { h.cleanup(); }
});
test('missing merchant login cannot become an enabled stock-maintenance job', async () => {
  const h = await harness();
  try {
    h.configs.set(production, catalog('device1', [42, 43]));
    await enableMaintenanceFixture(h, { unavailableMerchant: true });
    const result = await h.message({ type: 'maintain', sites: [selections(h)[1]], target_stock: 999 });
    assert.equal(result.ok, true);
    assert.equal(result.data.maintenance.running, 0);
    assert.equal(result.data.bindings[0].reason, 'no_tab');
    assert.equal(h.calls.some(call => call.url.endsWith('/claim')), false);
  } finally { h.cleanup(); }
});

test('pause all during setup also cancels bindings that were not yet saved', async () => {
  const h = await harness();
  try {
    h.configs.set(local, catalog('local-device', [101, 102]));
    h.configs.set(production, catalog('device1', [42, 43]));
    await enableMaintenanceFixture(h);
    const fetcher = globalThis.fetch;
    let release;
    let entered;
    const enteredConfig = new Promise(resolve => { entered = resolve; });
    const hold = new Promise(resolve => { release = resolve; });
    let delayed = false;
    globalThis.fetch = async (url, options) => {
      if (!delayed && url.endsWith('/config')) { delayed = true; entered(); await hold; }
      return fetcher(url, options);
    };
    const operation = h.message({ type: 'maintain', sites: selections(h), target_stock: 999 });
    await enteredConfig;
    await h.message({ type: 'pause-all' });
    release();
    await operation;
    const result = await h.message({ type: 'snapshot' });
    assert.equal(result.data.bindings.length, 4);
    assert.equal(result.data.bindings.some(binding => binding.enabled), false);
    assert.equal(h.calls.some(call => call.url.endsWith('/claim') || call.url.endsWith('/stock-target')), false);
  } finally { h.cleanup(); }
});

test('toolbar opens the single settings page', async () => {
  const h = await harness();
  try {
    let opened = 0;
    chrome.runtime.openOptionsPage = async () => { opened++; };
    await h.listeners.open();
    assert.equal(opened, 1);
  } finally { h.cleanup(); }
});
