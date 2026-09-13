const form = document.querySelector('#form');
const feedback = document.querySelector('#feedback');
async function request(message) {
  const response = await chrome.runtime.sendMessage(message);
  if (!response?.ok) throw new Error(response?.message || '操作失败');
  return response.data;
}
async function render() {
  const data = await request({ type: 'snapshot' });
  const list = document.querySelector('#bindings'); list.replaceChildren();
  for (const item of data.bindings) {
    const card = document.createElement('article');
    const title = document.createElement('h2'); title.textContent = item.name;
    const detail = document.createElement('p'); detail.textContent = `${item.site} · ${item.config_id} / ${item.device_id} · 商品 ${item.goods_id}`;
    const status = document.createElement('p'); status.className = 'help'; status.textContent = `${item.enabled ? '自动检查中' : item.reason_text} · 库存 ${item.stock ?? '未核验'} · 最后成功 ${item.lastSuccess ? new Date(item.lastSuccess).toLocaleString('zh-CN') : '尚无记录'}`;
    const remove = document.createElement('button'); remove.className = 'secondary'; remove.textContent = '移除此绑定'; remove.disabled = data.busy || item.enabled || item.pending;
    remove.addEventListener('click', async () => { try { await request({ type: 'remove', id: item.id }); await render(); } catch (error) { feedback.textContent = error.message; } });
    card.append(title, detail, status, remove); list.append(card);
  }
}
let inspected = null;
const inspectButton = document.querySelector('#inspect');
const submit = form.querySelector('button[type="submit"]');
for (const name of ['site', 'device_key']) form.elements[name].addEventListener('input', () => { inspected = null; submit.disabled = true; document.querySelector('#product-label').hidden = true; });
inspectButton.addEventListener('click', async () => {
  const binding = Object.fromEntries(new FormData(form));
  inspectButton.disabled = true;
  try {
    const origins = [binding.site === 'http://127.0.0.1:8080' ? 'http://127.0.0.1/*' : `${binding.site}/*`];
    if (!(await chrome.permissions.request({ origins }))) throw new Error('需要允许访问当前绑定站点');
    inspected = await request({ type: 'inspect', binding });
    form.elements.goods_id.replaceChildren();
    for (const product of inspected.products) {
      const option = document.createElement('option'); option.value = product.goods_id; option.textContent = `${product.title} · 商品 ${product.goods_id}`; form.elements.goods_id.append(option);
    }
    document.querySelector('#product-label').hidden = false;
    submit.disabled = !inspected.products.length;
    feedback.textContent = inspected.products.length ? '已读取设备允许的商品。请选择并保存。' : '该设备没有启用的商品。';
  } catch (error) { feedback.textContent = error.message; }
  finally { inspectButton.disabled = false; }
});
form.addEventListener('submit', async event => {
  event.preventDefault();
  if (!inspected) return;
  const binding = Object.fromEntries(new FormData(form));
  binding.device_id = inspected.device_id;
  binding.name = inspected.products.find(product => String(product.goods_id) === binding.goods_id)?.title || '';
  submit.disabled = true;
  try {
    await request({ type: 'bind', binding });
    form.elements.device_key.value = '';
    inspected = null;
    document.querySelector('#product-label').hidden = true;
    feedback.textContent = '绑定已核对。请在弹窗中检查库存，再手动开始。';
    await render();
  } catch (error) { feedback.textContent = error.message; submit.disabled = false; }
});
await render();
