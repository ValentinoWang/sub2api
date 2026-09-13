import { RestockError, validateSite, fail } from './core.js';

export function validateDeviceKey(key) {
  if (typeof key !== 'string' || !/^ldxpd_[a-f0-9]{64}$/.test(key)) fail('binding_mismatch');
}
export function createBackend(binding, fetcher = fetch) {
  const site = validateSite(binding.site);
  validateDeviceKey(binding.device_key);
  const call = async (path, body) => {
    let response;
    try {
      response = await fetcher(`${site}/api/v1/ldxp/device${path}`, {
        method: body === undefined ? 'GET' : 'POST', redirect: 'error', credentials: 'omit', cache: 'no-store',
        headers: { Authorization: `Bearer ${binding.device_key}`, 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(20_000),
      });
    } catch { throw new RestockError('backend_error'); }
    if (!response.ok) fail('backend_error');
    let result;
    try { result = await response.json(); } catch { fail('protocol_error'); }
    if (result?.code !== 0 || !result.data || typeof result.data !== 'object') fail('backend_error');
    return result.data;
  };
  const normalize = batch => batch ? { ...batch, config_id: binding.config_id, device_id: binding.device_id } : null;
  const config = async () => {
    const data = await call('/config');
    if (!data.device || !['string', 'number'].includes(typeof data.device.id) || data.device.revoked || !Array.isArray(data.products)) fail('binding_mismatch');
    return data;
  };
  return {
    config,
    resume: async () => {
      const result = await call('/resume', {});
      if (!result.enabled || result.paused_reason) fail('backend_error');
    },
    verifyBinding: async expected => {
      const data = await config();
      const product = data.products.find(product => product.goods_id === expected.goods_id && product.enabled);
      if (String(data.device.id) !== expected.device_id || expected.config_id !== 'ldxp' || !product) fail('binding_mismatch');
      return data;
    },
    reconcile: async (expected, report, batchID) => {
      if (batchID !== undefined && (typeof batchID !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(batchID))) fail('protocol_error');
      await call('/heartbeat', { authorization: 'verified' });
      const result = await call('/inventory', { goods_id: expected.goods_id, complete: report.complete, total: report.total, hashes: report.hashes, ...(batchID === undefined ? {} : { batch_id: batchID }) });
      if (typeof result.blocked !== 'boolean' || typeof result.batch_resolved !== 'boolean' || typeof result.identity_verified !== 'boolean') fail('protocol_error');
      return { ...result, blocked: result.blocked || !result.identity_verified, pending_batch: normalize(result.pending_batch) };
    },
    claim: async expected => {
      const { batch } = await call('/claim', { goods_id: expected.goods_id });
      return normalize(batch);
    },
    start: async (_expected, id) => {
      if (!/^[A-Za-z0-9_-]{1,128}$/.test(id)) fail('protocol_error');
      const result = await call(`/batches/${id}/start`, {});
      if (result.batch?.batch_id !== id || result.batch?.goods_id !== binding.goods_id || result.batch?.status !== 'uncertain') fail('protocol_error');
    },
    report: async (id, outcome) => {
      if (!/^[A-Za-z0-9_-]{1,128}$/.test(id) || !['imported', 'uncertain', 'authorization_failed'].includes(outcome)) fail('protocol_error');
      await call(`/batches/${id}/result`, { outcome });
    },
    authorizationFailed: async () => call('/heartbeat', { authorization: 'failed' }),
  };
}
