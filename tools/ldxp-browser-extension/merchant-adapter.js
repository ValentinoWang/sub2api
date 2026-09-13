// This function is injected directly by the extension into a selected top-level
// merchant tab. It has no message listener and never receives the device key.
export async function merchantOperation(operation, goodsID, codes = []) {
  const reject = code => { throw new Error(code); };
  try {
    if (location.origin !== 'https://www.ldxp.cn' || !location.pathname.startsWith('/merchant')) reject('no_tab');
    if (!navigator.onLine) reject('offline');
    if (!Number.isSafeInteger(goodsID) || goodsID < 1 || !['inventory', 'upload'].includes(operation)) reject('binding_mismatch');
    let saved;
    try { saved = JSON.parse(localStorage.getItem('auth-token')); } catch { reject('login_required'); }
    if (!saved || typeof saved.value !== 'string' || !saved.value || typeof saved.expiry !== 'number' || saved.expiry <= Date.now()) reject('login_required');
    const post = async (path, body) => {
      const response = await fetch(`${location.origin}${path}`, {
        method: 'POST', credentials: 'include', redirect: 'error', cache: 'no-store',
        headers: { 'Content-Type': 'application/json', 'Merchant-Token': saved.value },
        body: JSON.stringify(body), signal: AbortSignal.timeout(20_000),
      });
      if ([401, 403].includes(response.status)) reject('login_required');
      if (!response.ok) reject('merchant_error');
      const result = await response.json();
      if (result?.code === 401) reject('login_required');
      if (result?.code !== 1) reject('merchant_error');
      return result.data;
    };
    if (operation === 'upload') {
      if (!Array.isArray(codes) || codes.length < 1 || codes.length > 500 || new Set(codes).size !== codes.length || codes.some(code => typeof code !== 'string' || !code || code.length > 4096 || code.trim() !== code || /[\r\n]/.test(code))) reject('protocol_error');
      await post('/merchantApi/GoodsCardStorage/add', { goods_id: goodsID, content: codes.join('\n'), first: 0, remove_repeat: 1 });
      return { ok: true };
    }
    const hash = async value => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value))), byte => byte.toString(16).padStart(2, '0')).join('');
    const scan = async () => {
      const hashes = new Set();
      const rowIDs = new Set();
      let total = null;
      for (let current = 1; current <= 1000; current++) {
        const data = await post('/merchantApi/goodsCardStorage/list', { goods_id: goodsID, current, pageSize: 100, status: '0', first: '', keywords: '' });
        if (!data || !Number.isSafeInteger(data.total) || data.total < 0 || data.total > 100_000 || !Array.isArray(data.list)) reject('inventory_invalid');
        if (total !== null && data.total !== total) reject('inventory_invalid');
        total = data.total;
        if (data.list.length === 0 && hashes.size !== total) reject('inventory_invalid');
        for (const row of data.list) {
          // Exact field names are pinned to the observed merchant list contract.
          const code = row?.secret;
          if (!row || !['string', 'number'].includes(typeof row.id) || String(row.id) === '' || typeof code !== 'string' || !code || code.trim() !== code || /[\r\n]/.test(code)) reject('inventory_invalid');
          if (row.goods_id !== undefined && String(row.goods_id) !== String(goodsID)) reject('binding_mismatch');
          if (row.status !== undefined && String(row.status) !== '0') reject('inventory_invalid');
          const digest = await hash(code);
          if (rowIDs.has(String(row.id)) || hashes.has(digest)) reject('inventory_invalid');
          rowIDs.add(String(row.id));
          hashes.add(digest);
        }
        if (hashes.size > total) reject('inventory_invalid');
        if (hashes.size === total) return { complete: true, total, hashes: [...hashes].sort() };
      }
      reject('inventory_invalid');
    };
    const first = await scan();
    const second = await scan();
    if (first.total !== second.total || first.hashes.some((value, index) => value !== second.hashes[index])) reject('inventory_invalid');
    return { ok: true, inventory: second };
  } catch (error) {
    const allowed = ['no_tab', 'offline', 'binding_mismatch', 'login_required', 'merchant_error', 'inventory_invalid', 'protocol_error'];
    return { ok: false, error: allowed.includes(error?.message) ? error.message : 'merchant_error' };
  }
}
