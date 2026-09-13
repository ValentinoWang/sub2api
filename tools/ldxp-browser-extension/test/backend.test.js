import test from 'node:test';
import assert from 'node:assert/strict';
import { createBackend, validateDeviceKey } from '../backend.js';
const binding = { site: 'http://127.0.0.1:8080', config_id: 'ldxp', device_id: 'device1', goods_id: 42, device_key: `ldxpd_${'a'.repeat(64)}` };
const response = data => ({ ok: true, json: async () => ({ code: 0, data }) });
test('admin keys are rejected before making any request', () => {
  for (const key of ['sk-admin', 'Merchant-Token', 'a'.repeat(64), 'ldxpd_short']) assert.throws(() => validateDeviceKey(key));
});
test('backend inventory has hashes only and device credentials cannot redirect or use cookies', async () => {
  const calls = [];
  const backend = createBackend(binding, async (url, options) => {
    calls.push({ url, options });
    assert.equal(options.redirect, 'error'); assert.equal(options.credentials, 'omit');
    assert.deepEqual(Object.keys(options.headers).sort(), ['Authorization', 'Content-Type']);
    assert.equal(options.headers.Authorization, `Bearer ${binding.device_key}`);
    return response(url.endsWith('/heartbeat') ? {} : { identity_verified: true, blocked: false, batch_resolved: false, pending_batch: null });
  });
  await backend.reconcile(binding, { total: 1, complete: true, hashes: ['b'.repeat(64)] });
  assert.deepEqual(JSON.parse(calls[1].options.body), { goods_id: 42, total: 1, complete: true, hashes: ['b'.repeat(64)] });
  assert.equal(calls[1].url, 'http://127.0.0.1:8080/api/v1/ldxp/device/inventory');
});
test('binding validation rejects device or product mismatch', async () => {
  const backend = createBackend(binding, async () => response({ device: { id: 'other' }, products: [{ goods_id: 42, enabled: true }] }));
  await assert.rejects(backend.verifyBinding(binding), /binding_mismatch/);
});
test('even blocked=false cannot make identity_verified=false eligible for claim', async () => {
  const backend = createBackend(binding, async url => response(url.endsWith('/heartbeat') ? {} : { identity_verified: false, blocked: false, batch_resolved: false, pending_batch: null }));
  assert.equal((await backend.reconcile(binding, { complete: true, total: 0, hashes: [] })).blocked, true);
});
test('malformed or rejected start result cannot trigger upload', async () => {
  const backend = createBackend(binding, async () => response({ batch: { batch_id: 'wrong', goods_id: 42, status: 'uncertain' } }));
  await assert.rejects(backend.start(binding, 'batch1'), /protocol_error/);
});
test('inventory recovery sends only the validated original batch identifier', async () => {
  let requests = 0;
  const backend = createBackend(binding, async (url, options) => {
    requests++;
    if (url.endsWith('/inventory')) {
      assert.deepEqual(JSON.parse(options.body), { goods_id: 42, complete: true, total: 0, hashes: [], batch_id: 'original-batch' });
    }
    return response(url.endsWith('/heartbeat') ? {} : { identity_verified: true, blocked: false, batch_resolved: true, pending_batch: null });
  });
  const result = await backend.reconcile(binding, { complete: true, total: 0, hashes: [] }, 'original-batch');
  assert.equal(result.batch_resolved, true);
  assert.equal(requests, 2);
  for (const batchID of [null, 42, '', '../other', 'a'.repeat(129)]) await assert.rejects(backend.reconcile(binding, { complete: true, total: 0, hashes: [] }, batchID), /protocol_error/);
  assert.equal(requests, 2);
});
