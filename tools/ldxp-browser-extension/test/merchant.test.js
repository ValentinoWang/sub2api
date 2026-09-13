import test from 'node:test';
import assert from 'node:assert/strict';
import { merchantOperation } from '../merchant-adapter.js';
import { sha256 } from '../core.js';

async function withMerchant(callback, action) {
  const originals = Object.getOwnPropertyDescriptors(globalThis);
  Object.defineProperty(globalThis, 'location', { configurable: true, value: { origin: 'https://www.ldxp.cn', pathname: '/merchant/goods/list' } });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { onLine: true } });
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: { getItem: key => { assert.equal(key, 'auth-token'); return JSON.stringify({ value: 'browser-only-fixture', expiry: Date.now() + 60_000 }); } } });
  globalThis.fetch = callback;
  try { return await action(); } finally {
    for (const key of ['location', 'navigator', 'localStorage', 'fetch']) {
      if (originals[key]) Object.defineProperty(globalThis, key, originals[key]); else delete globalThis[key];
    }
  }
}
function response(data, code = 1) { return { ok: true, status: 200, json: async () => ({ code, data }) }; }
test('full pagination scans twice, returns hashes only, pins authenticated destination', async () => {
  const calls = [];
  const rows = Array.from({ length: 101 }, (_, i) => ({ id: i + 1, secret: `CODE-${i + 1}`, goods_id: 42, status: '0' }));
  const result = await withMerchant(async (url, options) => {
    assert.equal(url, 'https://www.ldxp.cn/merchantApi/goodsCardStorage/list');
    assert.equal(options.redirect, 'error');
    assert.equal(options.credentials, 'include');
    assert.equal(options.headers['Merchant-Token'], 'browser-only-fixture');
    const body = JSON.parse(options.body); calls.push(body.current);
    return response({ total: 101, list: rows.slice((body.current - 1) * 100, body.current * 100) });
  }, () => merchantOperation('inventory', 42));
  assert.deepEqual(calls, [1, 2, 1, 2]);
  assert.equal(result.inventory.hashes.length, 101);
  assert.ok(result.inventory.hashes.includes(await sha256('CODE-1')));
  assert.ok(!JSON.stringify(result).includes('CODE-'));
  assert.ok(!JSON.stringify(result).includes('browser-only-fixture'));
});
test('HTTP 200 with code401 is login failure and exposes no raw response', async () => {
  const result = await withMerchant(async () => response({ secret: 'private' }, 401), () => merchantOperation('inventory', 42));
  assert.deepEqual(result, { ok: false, error: 'login_required' });
});
test('missing secret, duplicate row, repeated pages, wrong goods, and changed scans fail closed', async () => {
  for (const rows of [[{ id: 1, content: 'not-secret-field' }], [{ id: 1, secret: 'A' }, { id: 1, secret: 'B' }], [{ id: 1, secret: 'A' }, { id: 2, secret: 'A' }], [{ id: 1, secret: 'A', goods_id: 999 }]]) {
    const result = await withMerchant(async () => response({ total: rows.length, list: rows }), () => merchantOperation('inventory', 42));
    assert.equal(result.ok, false);
  }
  let call = 0;
  const result = await withMerchant(async () => response({ total: 1, list: [{ id: ++call, secret: `C${call}` }] }), () => merchantOperation('inventory', 42));
  assert.deepEqual(result, { ok: false, error: 'inventory_invalid' });
});
test('empty inventory requires explicit list and total; total-only is rejected', async () => {
  const result = await withMerchant(async () => response({ total: 0 }), () => merchantOperation('inventory', 42));
  assert.equal(result.error, 'inventory_invalid');
});
test('upload is one exact request and newline injection is rejected before fetch', async () => {
  let calls = 0;
  const result = await withMerchant(async (url, options) => {
    calls++;
    assert.equal(url, 'https://www.ldxp.cn/merchantApi/GoodsCardStorage/add');
    assert.deepEqual(JSON.parse(options.body), { goods_id: 42, content: 'A\nB', first: 0, remove_repeat: 1 });
    return response({});
  }, async () => {
    const ok = await merchantOperation('upload', 42, ['A', 'B']);
    const bad = await merchantOperation('upload', 42, ['A\nB']);
    assert.equal(bad.error, 'protocol_error'); return ok;
  });
  assert.equal(calls, 1);
  assert.equal(result.ok, true);
});
