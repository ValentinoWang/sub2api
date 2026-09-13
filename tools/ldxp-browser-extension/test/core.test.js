import test from 'node:test';
import assert from 'node:assert/strict';
import { createBackend } from '../backend.js';
import { bindingID, initialState, runCycle, sha256, validateSite } from '../core.js';

const binding = { site: 'https://ai.rest2build.lol', config_id: 'shop', device_id: 'chrome1', goods_id: 42 };
async function fixture() {
  const state = { ...initialState(), reason: 'manual', enabled: true };
  const batch = { batch_id: 'batch1', config_id: 'shop', device_id: 'chrome1', goods_id: 42, status: 'claimed', codes: ['CODE-ONE'], code_hashes: [await sha256('CODE-ONE')] };
  const events = [];
  let uploaded = false;
  const inventory = () => ({ complete: true, total: uploaded ? 1 : 0, hashes: uploaded ? batch.code_hashes : [] });
  const args = {
    binding, state, now: () => 1000,
    save: async value => events.push(`save:${value.phase}:${value.batch?.status || '-'}`),
    merchant: { inventory: async () => inventory(), upload: async () => { events.push('upload'); uploaded = true; } },
    backend: {
      reconcile: async () => ({ batch_resolved: uploaded, blocked: false }),
      claim: async () => { events.push('claim'); return batch; },
      start: async () => { events.push('start'); },
    },
  };
  return { args, state, batch, events };
}
test('scope keys separate sites, configurations, devices and goods', () => {
  const original = bindingID(binding);
  for (const patch of [{ site: 'http://127.0.0.1:8080' }, { config_id: 'other' }, { device_id: 'other' }, { goods_id: 43 }]) assert.notEqual(bindingID({ ...binding, ...patch }), original);
  for (const site of ['https://evil.test', 'https://ai.rest2build.lol.evil.test', 'http://127.0.0.1:9999', 'https://ai.rest2build.lol/']) assert.throws(() => validateSite(site));
});
test('persist uncertain before start request and only resolve after identity reconciliation', async () => {
  const { args, state, events } = await fixture();
  await runCycle(args);
  assert.ok(events.indexOf('save:uploading:uncertain') < events.indexOf('start'));
  assert.ok(events.indexOf('start') < events.indexOf('upload'));
  assert.equal(state.batch, null);
  assert.equal(state.stock, 1);
  assert.equal(state.lastSuccess, 1000);
});
test('lost upload response never reuploads the batch on reconnect', async () => {
  const { args, state, events } = await fixture();
  args.merchant.upload = async () => { events.push('upload'); throw new Error('lost-response'); };
  await runCycle(args);
  assert.equal(state.enabled, false);
  assert.equal(state.reason, 'uncertain');
  args.backend.reconcile = async () => ({ blocked: false, batch_resolved: false });
  await runCycle({ ...args, mode: 'check' });
  assert.equal(events.filter(e => e === 'upload').length, 1);
  assert.equal(events.filter(e => e === 'claim').length, 1);
  assert.equal(state.reason, 'uncertain');
});
test('lost start response is also uncertain even if merchant upload never ran', async () => {
  const { args, state, events } = await fixture();
  args.backend.start = async () => { throw new Error('timeout'); };
  await runCycle(args);
  assert.equal(state.batch.status, 'uncertain');
  assert.ok(!events.includes('upload'));
});
test('checking after login recovery never resumes or uploads', async () => {
  const { args, state, events } = await fixture();
  state.enabled = false; state.reason = 'login_required';
  await runCycle({ ...args, mode: 'check' });
  assert.equal(state.checked, true);
  assert.equal(state.enabled, false);
  assert.ok(!events.includes('claim'));
  assert.ok(!events.includes('upload'));
});
test('sleep gap pauses before talking to the merchant', async () => {
  const { args, state, events } = await fixture();
  state.lastWake = 1;
  args.now = () => 200_001;
  args.merchant.inventory = async () => assert.fail('must not scan on silent automatic resume');
  await runCycle(args);
  assert.equal(state.reason, 'sleep');
  assert.ok(!events.includes('claim'));
});
test('total alone, duplicate hashes and partial inventory all block claims', async () => {
  for (const report of [{ total: 0 }, { complete: false, total: 0, hashes: [] }, { complete: true, total: 1, hashes: [] }, { complete: true, total: 2, hashes: ['a'.repeat(64), 'a'.repeat(64)] }]) {
    const { args, state, events } = await fixture();
    args.merchant.inventory = async () => report;
    await runCycle(args);
    assert.equal(state.reason, 'inventory_invalid');
    assert.ok(!events.includes('claim'));
  }
});
test('wrong product and mismatched code digest never reach upload', async () => {
  for (const mutate of [batch => { batch.goods_id = 43; }, batch => { batch.code_hashes = ['0'.repeat(64)]; }]) {
    const { args, batch, state, events } = await fixture(); mutate(batch);
    await runCycle(args);
    assert.ok(!events.includes('start'));
    assert.ok(!events.includes('upload'));
    assert.equal(state.enabled, false);
  }
});
test('claimed batch is reused and a different batch ID cannot replace it', async () => {
  const { args, state, events } = await fixture();
  state.batch = { batch_id: 'original', status: 'claimed' };
  await runCycle(args);
  assert.equal(state.reason, 'binding_mismatch');
  assert.equal(state.batch.batch_id, 'original');
  assert.ok(!events.includes('upload'));
});
test('only explicit start resumes the device; automatic checks cannot clear a server pause', async () => {
  for (const mode of ['automatic', 'check', 'start']) {
    const { args, events } = await fixture();
    args.backend.resume = async () => events.push('resume');
    await runCycle({ ...args, mode });
    assert.equal(events.includes('resume'), mode === 'start');
  }
});
test('pause requested before claim prevents issuance and leaves manual pause', async () => {
  const { args, state, events } = await fixture();
  await runCycle({ ...args, cancelled: () => true });
  assert.equal(state.reason, 'manual');
  assert.ok(!events.includes('claim'));
});
test('pause during claim retains claimed batch without starting upload', async () => {
  const { args, state, events, batch } = await fixture();
  let cancelled = false;
  args.backend.claim = async () => { cancelled = true; return batch; };
  await runCycle({ ...args, cancelled: () => cancelled });
  assert.equal(state.reason, 'manual');
  assert.equal(state.batch.batch_id, batch.batch_id);
  assert.equal(state.batch.status, 'claimed');
  assert.ok(!events.includes('start'));
  assert.ok(!events.includes('upload'));
});
test('lost successful reconciliation response resolves original batch from server terminal status', async () => {
  const { args, state, events } = await fixture();
  let scans = 0;
  args.backend.reconcile = async () => {
    scans++;
    if (scans === 2) throw new Error('lost server-verified response');
    return { identity_verified: true, blocked: false, batch_resolved: scans > 2, pending_batch: null };
  };
  await runCycle(args);
  assert.equal(state.reason, 'uncertain');
  assert.equal(state.batch.code_hashes.length, 1);
  assert.equal('codes' in state.batch, false);
  await runCycle({ ...args, mode: 'check' });
  assert.equal(state.batch, null);
  assert.equal(state.checked, true);
  assert.equal(state.enabled, false);
  assert.equal(events.filter(event => event === 'upload').length, 1);
});

test('server-confirmed batch recovers after lost response and subsequent sale', async () => {
  const { args, state, batch, events } = await fixture();
  batch.codes.push('CODE-TWO');
  batch.code_hashes.push(await sha256('CODE-TWO'));
  args.binding = { ...args.binding, device_key: `ldxpd_${'a'.repeat(64)}` };
  let uploaded = false;
  let sold = false;
  let terminalVerified = false;
  const inventoryRequests = [];
  args.merchant.upload = async () => { uploaded = true; events.push('upload'); };
  args.merchant.inventory = async () => {
    const hashes = uploaded ? (sold ? batch.code_hashes.slice(1) : batch.code_hashes) : [];
    return { complete: true, total: hashes.length, hashes };
  };
  args.backend = createBackend(args.binding, async (url, options) => {
    const body = options.body && JSON.parse(options.body);
    const success = data => ({ ok: true, json: async () => ({ code: 0, data }) });
    if (url.endsWith('/inventory')) {
      inventoryRequests.push(body);
      if (uploaded && !terminalVerified) {
        terminalVerified = true;
        throw new Error('server committed verification but response was lost');
      }
      return success({ identity_verified: true, blocked: false, pending_batch: null, batch_resolved: terminalVerified && body.batch_id === batch.batch_id });
    }
    if (url.endsWith('/claim')) return success({ batch });
    if (url.endsWith('/start')) return success({ batch: { ...batch, status: 'uncertain' } });
    return success({});
  });
  await runCycle(args);
  assert.equal(state.reason, 'uncertain');
  sold = true;
  await runCycle({ ...args, mode: 'check' });
  assert.equal(inventoryRequests.at(-1).total, 1);
  assert.equal(inventoryRequests.at(-1).batch_id, batch.batch_id);
  assert.equal(state.batch, null);
  assert.equal(state.checked, true);
  assert.equal(state.enabled, false);
  assert.equal(events.filter(event => event === 'upload').length, 1);
});
test('sale before any server verification remains uncertain and cannot issue again', async () => {
  const { args, state, batch, events } = await fixture();
  state.batch = { batch_id: batch.batch_id, status: 'uncertain', code_hashes: batch.code_hashes };
  state.enabled = false;
  args.backend.reconcile = async (_binding, _inventory, batchID) => {
    assert.equal(batchID, batch.batch_id);
    return { identity_verified: true, blocked: true, pending_batch: { ...batch, status: 'uncertain' }, batch_resolved: false };
  };
  await runCycle({ ...args, mode: 'check' });
  assert.equal(state.batch.batch_id, batch.batch_id);
  assert.equal(state.reason, 'uncertain');
  assert.equal(state.checked, false);
  assert.equal(state.enabled, false);
  assert.ok(!events.includes('claim'));
  assert.ok(!events.includes('upload'));
});
test('inventory hash presence alone cannot override unresolved server batch status', async () => {
  const { args, state, batch, events } = await fixture();
  state.batch = { batch_id: batch.batch_id, status: 'uncertain', code_hashes: batch.code_hashes };
  state.enabled = false;
  args.merchant.inventory = async () => ({ complete: true, total: 1, hashes: batch.code_hashes });
  args.backend.reconcile = async () => ({ identity_verified: true, blocked: false, pending_batch: null, batch_resolved: false });
  await runCycle({ ...args, mode: 'check' });
  assert.equal(state.batch.batch_id, batch.batch_id);
  assert.equal(state.checked, false);
  assert.equal(state.reason, 'uncertain');
  assert.ok(!events.includes('claim'));
});
