const feedback = document.querySelector('#feedback');
async function request(message) {
  const result = await chrome.runtime.sendMessage(message);
  if (!result?.ok) throw new Error(result?.message || '无法读取扩展状态');
  return result.data;
}
function line(list, label, value) {
  const dt = document.createElement('dt'); dt.textContent = label;
  const dd = document.createElement('dd'); dd.textContent = value;
  list.append(dt, dd);
}
function render(data) {
  const list = document.querySelector('#bindings');
  list.replaceChildren();
  if (!data.bindings.length) { const empty = document.createElement('p'); empty.className = 'empty'; empty.textContent = '尚未绑定商品。请先打开「管理绑定」。'; list.append(empty); }
  for (const item of data.bindings) {
    const card = document.querySelector('#binding').content.cloneNode(true);
    card.querySelector('h2').textContent = item.name || `商品 ${item.goods_id}`;
    const badge = card.querySelector('.badge');
    badge.textContent = item.enabled ? '自动检查中' : '已暂停';
    badge.classList.toggle('running', item.enabled);
    const dl = card.querySelector('dl');
    line(dl, '站点', item.site);
    line(dl, '绑定', `${item.config_id} / ${item.device_id}`);
    line(dl, '商品', String(item.goods_id));
    line(dl, '库存', item.stock === null ? '尚未核验' : `${item.stock} 个未售`);
    line(dl, '成功检查', item.lastSuccess ? new Date(item.lastSuccess).toLocaleString('zh-CN') : '尚无记录');
    card.querySelector('.reason').textContent = item.reason_text;
    for (const button of card.querySelectorAll('[data-action]')) {
      const type = button.dataset.action;
      button.disabled = (data.busy && type !== 'pause') || (type === 'start' && (!item.checked || item.enabled)) || (type === 'pause' && !item.enabled && !data.busy);
      button.addEventListener('click', async () => {
        button.disabled = true; feedback.textContent = type === 'check' ? '正在逐码核验库存…' : '正在处理…';
        try { render(await request({ type, id: item.id })); feedback.textContent = ''; }
        catch (error) { feedback.textContent = error.message; await refresh(false); }
      });
    }
    list.append(card);
  }
}
async function refresh(clear = true) {
  try { render(await request({ type: 'snapshot' })); if (clear) feedback.textContent = ''; }
  catch (error) { feedback.textContent = error.message; }
}
document.querySelector('#refresh').addEventListener('click', () => refresh());
document.querySelector('#options').addEventListener('click', () => chrome.runtime.openOptionsPage());
chrome.storage.onChanged.addListener(() => refresh(false));
await refresh();
