import test from 'node:test';
import assert from 'node:assert/strict';
import { bindingID, initialState } from '../core.js';

async function harness(statePatch = {}) {
  const binding = { site: 'https://ai.rest2build.lol', config_id: 'ldxp', device_id: 'device1', goods_id: 42, name: '5元额度', device_key: `ldxpd_${'f'.repeat(64)}` };
  const id = bindingID(binding);
  const data = { bindings: { [id]: binding }, states: { [id]: { ...initialState(), ...statePatch } } };
  const listeners = {}; const access = []; const alarms = [];
  const previousChrome = globalThis.chrome;
  globalThis.chrome = {
    storage: { local: {
      setAccessLevel: async value => access.push(value),
      get: async () => structuredClone(data),
      set: async value => Object.assign(data, structuredClone(value)),
    } },
    alarms: { create: async (...args) => alarms.push(args), onAlarm: { addListener: listener => { listeners.alarm = listener; } } },
    runtime: { id: 'self', getURL: page => `chrome-extension://self/${page}`, onMessage: { addListener: listener => { listeners.message = listener; } }, onStartup: { addListener: listener => { listeners.startup = listener; } } },
    tabs: { query: async () => [] },
  };
  await import(`../worker.js?test=${Math.random()}`);
  const sender = { id: 'self', url: 'chrome-extension://self/options.html', origin: 'chrome-extension://self', tab: { url: 'chrome-extension://self/options.html' } };
  const message = payload => new Promise(resolve => listeners.message(payload, sender, resolve));
  return { id, data, access, alarms, listeners, message, cleanup: () => { globalThis.chrome = previousChrome; } };
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
    assert.equal(result.ok, false); assert.equal(result.error, 'interrupted');
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
